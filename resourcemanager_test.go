// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"github.com/bmizerany/assert"
	"testing"
	"time"
)

type FakeSigarGetter struct {
	memResident uint64
	procUsed    float64
	timeNow     float64
}

// Gets the Resident memory of a process.
func (s *FakeSigarGetter) getMemResident(pid int) (uint64, error) {
	return (s.memResident), nil
}

// Gets the proc time and a timestamp and returns a ProcUsedTimestamp.
func (s *FakeSigarGetter) getProcUsedTimestamp(pid int) (*ProcUsedTimestamp,
	error) {
	return &ProcUsedTimestamp{
		procUsed:      float64(s.procUsed),
		nanoTimestamp: s.timeNow,
	}, nil
}

var r ResourceManager

func init() {
	r = ResourceManager{}
}

func TestCalculateProcPercent(t *testing.T) {
	first := ProcUsedTimestamp{
		procUsed: 2886, nanoTimestamp: 1.342471447022575e+18}
	second := ProcUsedTimestamp{
		procUsed: 3849, nanoTimestamp: 1.342471449022077e+18}
	var data []interface{}
	data = append(data, first)
	data = append(data, second)
	resourceHolder := ResourceHolder{data: data}
	proc, err := resourceHolder.calculateProcPercent()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 0.48161999345784273, proc)
}

func TestGatherMem(t *testing.T) {
	interval, _ := time.ParseDuration("1s")
	duration, _ := time.ParseDuration("0s")
	r.SetSigarInterface(&FakeSigarGetter{memResident: 1024})
	pe := &ParsedEvent{
		resourceName: MEMORY_USED_NAME,
		interval:     interval,
		duration:     duration,
	}
	resourceVal, err := r.GetResource(pe, 1234)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, uint64(1024), resourceVal)
}

// Can't get proc percent with only one data point.
func TestGatherFirstProcPercentZero(t *testing.T) {
	interval, _ := time.ParseDuration("1s")
	duration, _ := time.ParseDuration("0s")
	r.SetSigarInterface(&FakeSigarGetter{procUsed: 2886,
		timeNow: 1.342471447022575e+18})
	pe := &ParsedEvent{
		resourceName: "cpu_percent",
		interval:     interval,
		duration:     duration,
	}
	resourceVal, err := r.GetResource(pe, 1234)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, float64(0), resourceVal)
}

// When we have gotten proc percent twice, then we can get the proc time.
func TestGatherProcPercent(t *testing.T) {
	interval, _ := time.ParseDuration("1s")
	duration, _ := time.ParseDuration("0s")
	fsg := FakeSigarGetter{procUsed: 2886, timeNow: 1.342471447022575e+18}
	r.SetSigarInterface(&fsg)
	pe := &ParsedEvent{
		resourceName: "cpu_percent",
		interval:     interval,
		duration:     duration,
	}
	resourceVal, err := r.GetResource(pe, 1234)
	if err != nil {
		t.Fatal(err)
	}
	fsg.procUsed = 3849
	fsg.timeNow = 1.342471449022077e+18
	r.SetSigarInterface(&fsg)
	resourceVal, err = r.GetResource(pe, 1234)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, float64(0.48161999345784273), resourceVal)
}

func TestParseAmountErrors(t *testing.T) {
	_, err := r.ParseAmount(MEMORY_USED_NAME, "2k")
	assert.Equal(t, "memory_used '2k' is not the correct format.", err.Error())

	_, err = r.ParseAmount(MEMORY_USED_NAME, "$kb")
	assert.Equal(t, "strconv.ParseUint: parsing \"$\": invalid syntax",
		err.Error())

	_, err = r.ParseAmount(MEMORY_USED_NAME, "5zb")
	assert.Equal(t, "Invalid units 'zb' on 'memory_used'.", err.Error())

	_, err = r.ParseAmount(CPU_PERCENT_NAME, "5zb")
	assert.Equal(t, "strconv.ParseFloat: parsing \"5zb\": invalid syntax",
		err.Error())
}

func TestIsValidResourceName(t *testing.T) {
	assert.Equal(t, true, r.IsValidResourceName(MEMORY_USED_NAME))
	assert.Equal(t, true, r.IsValidResourceName(CPU_PERCENT_NAME))
	assert.Equal(t, true, r.IsValidResourceName(CPU_PERCENT_NAME))
}

func TestSaveDataReusesSlice(t *testing.T) {
	oldMax := MAX_DATA_TO_STORE
	MAX_DATA_TO_STORE = 3
	rh := &ResourceHolder{}
	for i := 0; i < MAX_DATA_TO_STORE; i++ {
		rh.saveData(i)
	}
	assert.Equal(t, 0, rh.data[0])
	assert.Equal(t, MAX_DATA_TO_STORE-1, rh.data[len(rh.data)-1])
	rh.saveData(1337)
	assert.Equal(t, 1337, rh.data[0])
	rh.saveData(1338)
	assert.Equal(t, 1338, rh.data[1])
	MAX_DATA_TO_STORE = oldMax
}

func TestCircularProcData(t *testing.T) {
	oldMax := MAX_DATA_TO_STORE
	MAX_DATA_TO_STORE = 3
	pe := &ParsedEvent{
		resourceName: "cpu_percent",
	}
	baseTime := float64(1.342471447022575e+18)
	for i := 0; i < MAX_DATA_TO_STORE; i++ {
		timeNow := baseTime + float64(i*int(time.Millisecond))
		fsg := &FakeSigarGetter{
			procUsed: float64(2886 + i),
			timeNow:  timeNow,
		}
		r.SetSigarInterface(fsg)
		//  put in the fake sigarwrapper shit here
		_, err := r.GetResource(pe, 1234)
		if err != nil {
			t.Fatal(err)
		}
	}
	resourceHolders := r.resourceHolders
	var rh *ResourceHolder
	for _, resourceHolder := range resourceHolders {
		if resourceHolder.resourceName == "cpu_percent" {
			rh = resourceHolder
			break
		}
	}

	// Assert we have what we expect.
	assert.Equal(t, float64(2886), rh.data[0].(ProcUsedTimestamp).procUsed)
	assert.Equal(t, float64(2888),
		rh.data[len(rh.data)-1].(ProcUsedTimestamp).procUsed)

	// Now, make sure that we loop on the data slice.
	fsg := &FakeSigarGetter{
		procUsed: float64(2897),
		timeNow:  baseTime + float64(20*NANO_TO_MILLI),
	}
	r.SetSigarInterface(fsg)
	//  put in the fake sigarwrapper shit here
	resourceVal, err := r.GetResource(pe, 1234)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, float64(2897), rh.data[0].(ProcUsedTimestamp).procUsed)
	time1 := (float64(baseTime) + float64(2*NANO_TO_MILLI)) / NANO_TO_MILLI
	time2 := (float64(baseTime) + float64(20*NANO_TO_MILLI)) / NANO_TO_MILLI
	expected := float64(2897-2888) / (time2 - time1)
	assert.Equal(t, expected, resourceVal)
	MAX_DATA_TO_STORE = oldMax
}
