// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"fmt"
	"github.com/cloudfoundry/gosigar"
	"strconv"
	"time"
)

// TODO:
// - Cache resources so that we don't ask for the resource 2x for 2 rules using
//       it.
// - Start using durations to calculate resource avg over the time duration.
// - For right now we keep 5min of data for each resourceXpid.  Figure out what
//       the right thing to do is, especially when we have different rules
//       requesting the same resourceXpid at different intervals
// - Do we really need to allow for any other rule data than uint64?
// - Implement all sys stuff

// This interface allows us to mock sigar in unit tests.
type SigarInterface interface {
	getMemResident(pid int) (uint64, error)
	getProcTime(pid int) (float64, error)
}

type SigarGetter struct{}

// Gets the Resident memory of a process.
func (s *SigarGetter) getMemResident(pid int) (uint64, error) {
	mem := sigar.ProcMem{}
	if err := mem.Get(pid); err != nil {
		return 0, fmt.Errorf("Couldnt get mem for pid '%v'.", pid)
	}
	return mem.Resident, nil
}

// Gets the proc time and a timestamp and returns a DataTimestamp.
func (s *SigarGetter) getProcTime(pid int) (float64, error) {
	procTime := sigar.ProcTime{}
	if err := procTime.Get(pid); err != nil {
		return 0, fmt.Errorf("Couldnt get proctime for pid '%v'.", pid)
	}
	return float64(procTime.Total), nil
}

// Don't create more.
type ResourceManager struct {
	resourceHolders []*ResourceHolder
	sigarInterface  SigarInterface
}

type ResourceHolder struct {
	processName  string
	resourceName string
	dataTimestamps  []DataTimestamp
	firstEntryIndex int
}

var resourceManager ResourceManager = ResourceManager{
	sigarInterface: &SigarGetter{},
}

type DataTimestamp struct {
	data          interface{}
	nanoTimestamp float64
}

// Not const so it can be changed in tests.
var MAX_DATA_TO_STORE = 120

const NANO_TO_MILLI = float64(time.Millisecond)

const (
	CPU_PERCENT_NAME     = "cpu_percent"
	MEMORY_USED_NAME     = "memory_used"
)

var validResourceNames = map[string]bool{
	MEMORY_USED_NAME: true,
	CPU_PERCENT_NAME: true,
}

// Cleans data from ResourceManager.
func (r *ResourceManager) CleanData() {
	resourceManager.resourceHolders = []*ResourceHolder{}
}

// Get the nth entry in the data.  Accepts a negaitve number, as well, so that
// the last etc. can be referenced.
func (r *ResourceHolder) getNthData(index int) (*DataTimestamp, error) {
	data := r.dataTimestamps
	dataLen := len(data)
	if index < 0 {
		if index+dataLen < 0 {
			return nil, fmt.Errorf("Cannot have a negative index '%v' larger than "+
				"the data length '%v'.", index, dataLen)
		}
		if dataLen == MAX_DATA_TO_STORE {
			lastIndex := r.firstEntryIndex + index
			if lastIndex < 0 {
				lastIndex = dataLen + lastIndex
			}
			return &data[lastIndex], nil
		} else {
			return &data[dataLen+index], nil
		}
	} else {
		return &data[index], nil
	}
	panic("Can't get here.")
}

// Given an array of data which is of type ProcUsedTimestamp, will return the
// percent of proc time that was used.
func (r *ResourceHolder) calculateProcPercent() (float64, error) {
	data := r.dataTimestamps
	dataLen := len(data)
	if dataLen <= 1 {
		return 0, nil
	}
	last, err := r.getNthData(-1)
	if err != nil {
		return 0, err
	}
	secondLast, err := r.getNthData(-2)
	if err != nil {
		return 0, err
	}
	lastProc := last.data.(float64)
	secondLastProc := secondLast.data.(float64)
	lastMilli := last.nanoTimestamp / NANO_TO_MILLI
	secondLastMilli := secondLast.nanoTimestamp / NANO_TO_MILLI
	return (lastProc - secondLastProc) / (lastMilli - secondLastMilli), nil
}

// Given a ParsedEvent, will populate the correct resourceHolder with the
// resource value.
func (r *ResourceManager) gatherResource(parsedEvent *ParsedEvent,
	pid int) error {
	processName := parsedEvent.processName
	resourceName := parsedEvent.resourceName
	for _, resourceHolder := range r.resourceHolders {
		if resourceHolder.processName == processName &&
			resourceHolder.resourceName == resourceName {
			if err := resourceHolder.gather(pid, &r.sigarInterface); err != nil {
				return err
			}
			return nil
		}
	}
	resourceHolder := &ResourceHolder{
		processName:  processName,
		resourceName: resourceName,
	}
	if err := resourceHolder.gather(pid, &r.sigarInterface); err != nil {
		return err
	}
	r.resourceHolders = append(r.resourceHolders, resourceHolder)
	return nil
}

// Takes a ParsedEvent and pid and returns the value for the resource used in
// the rule.
func (r *ResourceManager) GetResource(parsedEvent *ParsedEvent,
	pid int) (interface{}, error) {
	resourceName := parsedEvent.resourceName
	processName := parsedEvent.processName
	if err := r.gatherResource(parsedEvent, pid); err != nil {
		return nil, err
	}
	for _, resourceHolder := range r.resourceHolders {
		lenResourceData := len(resourceHolder.dataTimestamps)
		if resourceHolder.processName == processName &&
			resourceHolder.resourceName == resourceName && lenResourceData > 0 {
			if resourceName == CPU_PERCENT_NAME {
				procPercent, err := resourceHolder.calculateProcPercent()
				if err != nil {
					return nil, err
				}
				return procPercent, nil
			} else if resourceName == MEMORY_USED_NAME {
				last, err := resourceHolder.getNthData(-1)
				if err != nil {
					return nil, err
				}
				return last.data, nil
			}
		}
	}
	return nil,
		fmt.Errorf("Could not find resource value for resource %v on process %v.",
			resourceName, processName)
}

// A utility function that parses a rule's amount string and returns the value
// as the correct type.  For instance, in a rule such as 'memory_used > 14mb'
// the amount is '14mb' which this function would turn into a uint64 of
// 14*1024*1024.
func (r *ResourceManager) ParseAmount(resourceName string,
	amount string) (interface{}, error) {
	if resourceName == MEMORY_USED_NAME {
		lenAmount := len(amount)
		if lenAmount < 3 {
			return nil, fmt.Errorf("%v '%v' is not the correct format.",
				resourceName, amount)
		}
		units := amount[lenAmount-2:]
		amount = amount[0 : lenAmount-2]
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
				units, resourceName)
		}
		return amountUi, nil
	} else if resourceName == CPU_PERCENT_NAME {
		amountUi, err := strconv.ParseFloat(amount, 64)
		return amountUi, err
	}
	return nil, fmt.Errorf("Unknown resource name %v.", resourceName)
}

// Saves a data point into the data array of the ResourceHolder.
func (r *ResourceHolder) saveData(dataToSave interface{}) {
	dataTimestamp := DataTimestamp{
		data:          dataToSave,
		nanoTimestamp: float64(time.Now().UnixNano()),
	}

	if len(r.dataTimestamps) > MAX_DATA_TO_STORE {
		panic("This shouldn't happen.")
	} else if len(r.dataTimestamps) == MAX_DATA_TO_STORE {
		// Clear out old data.
		r.dataTimestamps[r.firstEntryIndex] = dataTimestamp
		r.firstEntryIndex++
		if r.firstEntryIndex == MAX_DATA_TO_STORE {
			r.firstEntryIndex = 0
		}
	} else {
		r.dataTimestamps = append(r.dataTimestamps, dataTimestamp)
	}
}

// Gets the data for a resource and saves it to the ResourceHolder.
func (r *ResourceHolder) gather(pid int, sigarInterface *SigarInterface) error {
	if r.resourceName == MEMORY_USED_NAME {
		mem, err := (*sigarInterface).getMemResident(pid)
		if err != nil {
			return err
		}
		r.saveData(mem)
	} else if r.resourceName == CPU_PERCENT_NAME {
		procTime, err := (*sigarInterface).getProcTime(pid)
		if err != nil {
			return err
		}
		r.saveData(procTime)
	}
	return nil
}

// Checks to see if a resource is a valid resource.
func (r *ResourceManager) IsValidResourceName(resourceName string) bool {
	if _, hasKey := validResourceNames[resourceName]; hasKey {
		return true
	}
	return false
}

// Allows the sigar interface to be set so that tests can set it.
func (r *ResourceManager) SetSigarInterface(sigar SigarInterface) {
	r.sigarInterface = sigar
}
