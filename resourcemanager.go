// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"fmt"
	"github.com/cloudfoundry/gosigar"
	"math"
	"strconv"
	"time"
)

// Bug(lisbakke): figure out how we change maxdatatostore when two rules are using the same resource, because they'll both be using the same resourceholder.

// This interface allows us to mock sigar in unit tests.
type SigarInterface interface {
	getMemResident(pid int) (uint64, error)
	getProcTime(pid int) (uint64, error)
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
func (s *SigarGetter) getProcTime(pid int) (uint64, error) {
	procTime := sigar.ProcTime{}
	if err := procTime.Get(pid); err != nil {
		return 0, fmt.Errorf("Couldnt get proctime for pid '%v'.", pid)
	}
	return procTime.Total, nil
}

// Don't create more.
type ResourceManager struct {
	resourceHolders []*ResourceHolder
	sigarInterface  SigarInterface
	// Used by eventmonitor to cache resources so they don't get pulled multiple
	// times when multiple rules are being checked for the same resource.
	cachedResources map[string]uint64
}

type ResourceHolder struct {
	processName     string
	resourceName    string
	dataTimestamps  []*DataTimestamp
	firstEntryIndex int64
	maxDataToStore  int64
}

var resourceManager ResourceManager = ResourceManager{
	sigarInterface:  &SigarGetter{},
	cachedResources: map[string]uint64{},
}

type DataTimestamp struct {
	data          uint64
	nanoTimestamp int64
}

const (
	INTERVAL_MARGIN_ERR = 0.05
	NANO_TO_MILLI       = float64(time.Millisecond)
)

const (
	CPU_PERCENT_NAME = "cpu_percent"
	MEMORY_USED_NAME = "memory_used"
)

var validResourceNames = map[string]bool{
	MEMORY_USED_NAME: true,
	CPU_PERCENT_NAME: true,
}

// Cleans data from ResourceManager.
func (r *ResourceManager) CleanData() {
	r.resourceHolders = []*ResourceHolder{}
	r.ClearCachedResources()
}

// Cleans up the resource data used for a process's event monitors.
func (r *ResourceManager) CleanDataForProcess(p *Process) {
	for _, resourceHolder := range r.resourceHolders {
		if resourceHolder.processName == p.Name {
			resourceHolder.dataTimestamps = []*DataTimestamp{}
			resourceHolder.firstEntryIndex = 0
		}
	}
	r.ClearCachedResources()
}

// Get the nth entry in the data.  Accepts a negaitve number, as well, so that
// the last etc. can be referenced.
func (r *ResourceHolder) getNthData(index int64) (*DataTimestamp, error) {
	data := r.dataTimestamps
	dataLen := int64(len(data))
	if index < 0 {
		if index+dataLen < int64(0) {
			return nil, fmt.Errorf("Cannot have a negative index '%v' larger than "+
				"the data length '%v'.", index, dataLen)
		}
		if dataLen == r.maxDataToStore {
			lastIndex := r.firstEntryIndex + index
			if lastIndex < 0 {
				lastIndex = dataLen + lastIndex
			}
			return data[lastIndex], nil
		} else {
			return data[dataLen+index], nil
		}
	} else {
		return data[index], nil
	}
	panic("Can't get here.")
}

// Given an array of data which is of type ProcUsedTimestamp, will return the
// percent of proc time that was used.
func (r *ResourceHolder) calculateProcPercent(point1 *DataTimestamp,
	point2 *DataTimestamp) (uint64, error) {
	lastProc := float64(point1.data)
	secondLastProc := float64(point2.data)
	lastMilli := float64(point1.nanoTimestamp) / NANO_TO_MILLI
	secondLastMilli := float64(point2.nanoTimestamp) / NANO_TO_MILLI
	return uint64(100 * (lastProc - secondLastProc) / (lastMilli - secondLastMilli)), nil
}

// Gets a ResourceHolder, given a ParsedEvent.  Creates a new one if one doesn't
// exist.
func (r *ResourceManager) getResourceHolder(
	parsedEvent *ParsedEvent) *ResourceHolder {
	processName := parsedEvent.processName
	resourceName := parsedEvent.resourceName
	for _, resourceHolder := range r.resourceHolders {
		if resourceHolder.processName == processName &&
			resourceHolder.resourceName == resourceName {
			return resourceHolder
		}
	}
	resourceHolder := &ResourceHolder{
		processName:  processName,
		resourceName: resourceName,
	}
	interval := float64(parsedEvent.interval.Seconds())
	duration := float64(parsedEvent.duration.Seconds())
	if duration == 0.0 {
		duration = interval
	}

	resourceHolder.maxDataToStore = int64(math.Ceil(duration / interval))
	r.resourceHolders = append(r.resourceHolders, resourceHolder)
	return resourceHolder
}

// Sets an entry in the resources cache.
func (r *ResourceManager) setCachedResource(resourceName string, value uint64) {
	r.cachedResources[resourceName] = value
}

// Gets an entry in the resources cache.
func (r *ResourceManager) getCachedResource(
	resourceName string) (uint64, bool) {
	value, has_key := r.cachedResources[resourceName]
	return value, has_key
}

// Clears the resources cache.
func (r *ResourceManager) ClearCachedResources() {
	r.cachedResources = map[string]uint64{}
}

// Given a ParsedEvent, will populate the correct resourceHolder with the
// resource value.
func (r *ResourceManager) gatherResource(parsedEvent *ParsedEvent,
	pid int) error {
	resourceHolder := r.getResourceHolder(parsedEvent)
	if err := r.gather(pid, resourceHolder); err != nil {
		return err
	}
	return nil
}

// Returns the average of an array of DataTimestamps.
func averageDataTimestampArray(array []*DataTimestamp) uint64 {
	sum := uint64(0)
	for _, val := range array {
		sum += val.data
	}
	return sum / uint64(len(array))
}

// Gets all entries in a resource holder since a nanosecond unix timestamp.
func (r *ResourceHolder) getEntriesSince(
	nanosecondStart int64) []*DataTimestamp {
	entries := []*DataTimestamp{}
	index := int64(-1)
	for {
		dataTimestamp, err := r.getNthData(index)
		index--
		if err != nil {
			break
		}
		entries = append(entries, dataTimestamp)
		if nanosecondStart > dataTimestamp.nanoTimestamp {
			break
		}
	}
	return entries
}

// Given an array of DataTimestamp entries, and a few other parameters, this
// function will return the average over the duration.
func (r *ResourceHolder) getDurationData(entries []*DataTimestamp,
	duration time.Duration, interval time.Duration,
	resourceName string) (uint64, error) {
	first := entries[0]
	last := entries[len(entries)-1]
	marginErr := 1 - INTERVAL_MARGIN_ERR
	errDuration := int64(marginErr * float64(duration.Nanoseconds()))
	// Add interval because if we have 3 entries that are 1s interval then we have
	// 2s as time covered, but really it covers 3s.
	timeDataCovers := first.nanoTimestamp - last.nanoTimestamp +
		(interval.Nanoseconds())
	if timeDataCovers > errDuration {
		if resourceName == MEMORY_USED_NAME {
			return averageDataTimestampArray(entries), nil
		} else if resourceName == CPU_PERCENT_NAME {
			return r.calculateProcPercent(first, last)
		}
	}
	return 0, nil
}

// Gets the current data for an event.
func (r *ResourceHolder) getData(parsedEvent *ParsedEvent) (uint64, error) {
	resourceName := parsedEvent.resourceName
	duration := parsedEvent.duration
	interval := parsedEvent.interval
	if (duration.Seconds() / interval.Seconds()) > 1 {
		timeNow := time.Now().UnixNano()
		entries := r.getEntriesSince(timeNow - duration.Nanoseconds())
		if len(entries) <= 1 {
			return 0, nil
		}
		return r.getDurationData(entries, duration, interval, resourceName)
	} else {
		// If we're not dealing with a duration.
		data, err := r.getNthData(-1)
		return data.data, err
	}
	panic("Can't get here.")
}

// Takes a ParsedEvent and pid and returns the value for the resource used in
// the rule.
func (r *ResourceManager) GetResource(parsedEvent *ParsedEvent,
	pid int) (uint64, error) {
	resourceName := parsedEvent.resourceName
	processName := parsedEvent.processName

	data, has_key := r.getCachedResource(resourceName)
	if has_key {
		return data, nil
	}

	if err := r.gatherResource(parsedEvent, pid); err != nil {
		return 0, err
	}

	for _, resourceHolder := range r.resourceHolders {
		lenResourceData := len(resourceHolder.dataTimestamps)
		if resourceHolder.processName == processName &&
			resourceHolder.resourceName == resourceName && lenResourceData > 0 {
			data, err := resourceHolder.getData(parsedEvent)
			if err != nil {
				return 0, err
			}
			r.setCachedResource(resourceName, data)
			return data, nil
		}
	}
	return 0,
		fmt.Errorf("Could not find resource value for resource %v on process %v.",
			resourceName, processName)
}

// A utility function that parses a rule's amount string and returns the value
// as the correct type.  For instance, in a rule such as 'memory_used > 14mb'
// the amount is '14mb' which this function would turn into a uint64 of
// 14*1024*1024.
func (r *ResourceManager) ParseAmount(resourceName string,
	amount string) (uint64, error) {
	if resourceName == MEMORY_USED_NAME {
		lenAmount := len(amount)
		if lenAmount < 3 {
			return 0, fmt.Errorf("%v '%v' is not the correct format.",
				resourceName, amount)
		}
		units := amount[lenAmount-2:]
		amount = amount[0 : lenAmount-2]
		amountUi, err := strconv.ParseUint(amount, 10, 64)
		if err != nil {
			return 0, err
		}
		switch units {
		case "kb":
			amountUi *= 1024
		case "mb":
			amountUi *= 1024 * 1024
		case "gb":
			amountUi *= 1024 * 1024 * 1024
		default:
			return 0, fmt.Errorf("Invalid units '%v' on '%v'.",
				units, resourceName)
		}
		return amountUi, nil
	} else if resourceName == CPU_PERCENT_NAME {
		amountUi, err := strconv.ParseUint(amount, 10, 64)
		return amountUi, err
	}
	return 0, fmt.Errorf("Unknown resource name %v.", resourceName)
}

// Saves a data point into the data array of the ResourceHolder.
func (r *ResourceHolder) saveData(dataToSave uint64) {
	dataTimestamp := &DataTimestamp{
		data:          dataToSave,
		nanoTimestamp: time.Now().UnixNano(),
	}
	timestampLen := int64(len(r.dataTimestamps))
	if timestampLen > r.maxDataToStore {
		panic("This shouldn't happen.")
	} else if timestampLen == r.maxDataToStore {
		// Clear out old data.
		r.dataTimestamps[r.firstEntryIndex] = dataTimestamp
		r.firstEntryIndex++
		if r.firstEntryIndex == r.maxDataToStore {
			r.firstEntryIndex = 0
		}
	} else {
		r.dataTimestamps = append(r.dataTimestamps, dataTimestamp)
	}
}

// Gets the data for a resource and saves it to the ResourceHolder.
func (r *ResourceManager) gather(pid int,
	resourceHolder *ResourceHolder) error {
	if resourceHolder.resourceName == MEMORY_USED_NAME {
		mem, err := r.sigarInterface.getMemResident(pid)
		if err != nil {
			return err
		}
		resourceHolder.saveData(mem)
	} else if resourceHolder.resourceName == CPU_PERCENT_NAME {
		procTime, err := r.sigarInterface.getProcTime(pid)
		if err != nil {
			return err
		}
		resourceHolder.saveData(procTime)
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
