// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"fmt"
	"github.com/cloudfoundry/gosigar"
	"strconv"
	"time"
)

// TODO:
// - Cache resources so that we don't ask for the resource 2x for 2 rules using it.
// - Start using durations to calculate resource avg over the time duration.
// - For right now we keep 5min of data for each resourceXpid.  Figure out what
//       the right thing to do is, especially when we have different rules
//       requesting the same resourceXpid at different intervals

// This interface allows us to mock sigar in unit tests.
type SigarInterface interface {
	getMemResident(int) (uint64, error)
	getProcUsedTimestamp(int) (ProcUsedTimestamp, error)
}

var sigarInterface SigarInterface = SigarGetter{}

type SigarGetter struct{}

// Gets the Resident memory of a process.
func (s SigarGetter) getMemResident(pid int) (uint64, error) {
	mem := sigar.ProcMem{}
	if err := mem.Get(pid); err != nil {
		return 0, fmt.Errorf("Couldnt get mem for pid '%v'.", pid)
	}
	return mem.Resident, nil
}

// Gets the proc time and a timestamp and returns a ProcUsedTimestamp.
func (s SigarGetter) getProcUsedTimestamp(pid int) (ProcUsedTimestamp, error) {
	procTime := sigar.ProcTime{}
	if err := procTime.Get(pid); err != nil {
		return ProcUsedTimestamp{},
			fmt.Errorf("Couldnt get proctime for pid '%v'.", pid)
	}
	return ProcUsedTimestamp{
		procUsed:      float64(procTime.Total),
		nanoTimestamp: float64(time.Now().UnixNano()),
	}, nil
}

type ResourceManager struct {
	startedMonitoringTicker bool
	resourceHolders         []ResourceHolder
	sigarInterface          SigarInterface
}

type ResourceHolder struct {
	pid          int
	resourceName string
	data         []interface{}
}

type ProcUsedTimestamp struct {
	procUsed      float64
	nanoTimestamp float64
}

const FIVE_MIN_SECS = 300
const NANO_TO_MILLI = 1000000.0

const MEM_USED_NAME = "mem_used"
const PROC_PERCENT_NAME = "proc_percent"

var validResourceNames = map[string]bool{
	MEM_USED_NAME:     true,
	PROC_PERCENT_NAME: true,
}

// Given an array of data which is of type ProcUsedTimestamp, will return the
// percent of proc time that was used.
func calculateProcPercent(data []interface{}) float64 {
	if len(data) <= 1 {
		return 0
	}
	last := data[len(data)-1].(ProcUsedTimestamp)
	secondLast := data[len(data)-2].(ProcUsedTimestamp)
	lastMilli := last.nanoTimestamp / NANO_TO_MILLI
	secondLastMilli := secondLast.nanoTimestamp / NANO_TO_MILLI
	return (last.procUsed - secondLast.procUsed) / (lastMilli - secondLastMilli)
}

// Given a pid, resource name and interval, will populate the correct
// resourceHolder with the resource value.
func (r *ResourceManager) gatherResource(pid int, resourceName string,
	interval time.Duration) error {
	for index, resourceHolder := range r.resourceHolders {
		if resourceHolder.pid == pid &&
			resourceHolder.resourceName == resourceName {
			if err := resourceHolder.gather(interval); err != nil {
				return err
			}
			r.resourceHolders[index] = resourceHolder
			return nil
		}
	}
	resourceHolder := ResourceHolder{pid: pid, resourceName: resourceName}
	resourceHolder.gather(interval)
	r.resourceHolders = append(r.resourceHolders, resourceHolder)
	return nil
}

// Takes a pid, resource name, interval, duration and returns the the resource's
// value.  Pid and resource name are used to identify which process and resource
// are needed, interval is just used to determine how much data to keep stored
// (5mins / interval = amount of data we keep), and duration is used to
// determine whether we are going to return the resource value at one moment or
// an average across a time period.
func (r *ResourceManager) GetResource(pid int, resourceName string,
	interval time.Duration, duration time.Duration) (interface{}, error) {

	if err := r.gatherResource(pid, resourceName, interval); err != nil {
		return nil, err
	}
	for _, resourceHolder := range r.resourceHolders {
		lenResourceData := len(resourceHolder.data)
		if resourceHolder.pid == pid &&
			resourceHolder.resourceName == resourceName && lenResourceData > 0 {
			if resourceName == PROC_PERCENT_NAME {
				return calculateProcPercent(resourceHolder.data), nil
			} else if resourceName == MEM_USED_NAME {
				return resourceHolder.data[lenResourceData-1], nil
			}
		}
	}
	return nil,
		fmt.Errorf("Could not find resource value for resource %v on pid %v.",
			resourceName, pid)
}

// A utility function that parses a rule's amount string and returns the value
// as the correct type.  For instance, in a rule such as 'mem_used > 14mb' the
// amount is '14mb' which this function would turn into a uint64 of 14*1024*1024.
func (r ResourceManager) ParseAmount(resourceName string,
	amount string) (interface{}, error) {
	if resourceName == MEM_USED_NAME {
		if len(amount) < 3 {
			return nil, fmt.Errorf("%v '%v' is not the correct format.",
				MEM_USED_NAME, amount)
		}
		units := amount[len(amount)-2:]
		amount = amount[0 : len(amount)-2]
		amountUi, err := strconv.ParseUint(amount, 10, 64)
		if err != nil {
			return nil, err
		}
		switch units {
		case "kb":
			amountUi *= 1024
		case "mb":
			amountUi *= 1024 * 1024
		case "gb":
			amountUi *= 1024 * 1024 * 1024
		default:
			return nil, fmt.Errorf("Invalid units '%v' on '%v'.",
				units, MEM_USED_NAME)
		}
		return amountUi, nil
	}
	if resourceName == PROC_PERCENT_NAME {
		amountUi, err := strconv.ParseFloat(amount, 64)
		return amountUi, err
	}
	return nil, fmt.Errorf("Unknown resource name %v.", resourceName)
}

// Saves a data point into the data array of the ResourceHolder.
func (r *ResourceHolder) saveData(dataToSave interface{},
	intervalAmount time.Duration) {
	numDataToKeep := int(FIVE_MIN_SECS / intervalAmount.Seconds())
	if len(r.data) >= numDataToKeep {
		// Clear out old data.
		amountToClear := len(r.data) - numDataToKeep
		r.data = append(r.data[amountToClear:], dataToSave)
	} else {
		r.data = append(r.data, dataToSave)
	}
}

// Gets the data for a resource and saves it to the ResourceHolder.  The
// intervalAmount is used to know how much data to store and how much to kick out.
func (r *ResourceHolder) gather(intervalAmount time.Duration) error {
	if r.resourceName == MEM_USED_NAME {
		memResident, err := sigarInterface.getMemResident(r.pid)
		if err != nil {
			return err
		}
		r.saveData(memResident, intervalAmount)
	}
	if r.resourceName == PROC_PERCENT_NAME {
		procUsedTimestamp, err := sigarInterface.getProcUsedTimestamp(r.pid)
		if err != nil {
			return err
		}
		r.saveData(procUsedTimestamp, intervalAmount)
	}
	return nil
}

// Checks to see if a resource is a valid resource.
func (r ResourceManager) IsValidResourceName(resourceName string) bool {
	if _, hasKey := validResourceNames[resourceName]; hasKey {
		return true
	}
	return false
}

// Allows the sigar interface to be set so that tests can set it.
func (r ResourceManager) SetSigarInterface(sigar SigarInterface) {
	sigarInterface = sigar
}
