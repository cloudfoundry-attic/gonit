// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"github.com/bmizerany/assert"
	"os"
	"testing"
	"time"
)

type FakeSigarGetter struct {
	memResident uint64
	procUsed    uint64
	sysMemUsed  uint64
}

func (s *FakeSigarGetter) getMemResident(pid int) (uint64, error) {
	return (s.memResident), nil
}

// Gets the proc time and a timestamp and returns a DataTimestamp.
func (s *FakeSigarGetter) getProcTime(pid int) (uint64, error) {
	return s.procUsed, nil
}

var r ResourceManager

func Setup() {
	r = ResourceManager{sigarInterface: &SigarGetter{}}
}

func TestCalculateProcPercent(t *testing.T) {
	Setup()
	first := &DataTimestamp{
		data: 2886, nanoTimestamp: 1.342471447022575e+18}
	second := &DataTimestamp{
		data: 3849, nanoTimestamp: 1.342471449022077e+18}
	var data []*DataTimestamp
	data = append(data, first)
	data = append(data, second)
	resourceHolder := ResourceHolder{dataTimestamps: data, maxDataToStore: 3}
	proc, err := resourceHolder.calculateProcPercent(second, first)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, uint64(48), proc)
}

func TestGatherMem(t *testing.T) {
	Setup()
	duration, _ := time.ParseDuration("0s")
	interval, _ := time.ParseDuration("1s")
	r.SetSigarInterface(&FakeSigarGetter{memResident: 1024})
	pe := &ParsedEvent{
		resourceName: MEMORY_USED_NAME,
		duration:     duration,
		interval:     interval,
	}
	resourceVal, err := r.GetResource(pe, 1234)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, uint64(1024), resourceVal)
}

func TestgetMemResident(t *testing.T) {
	Setup()
	pid := os.Getpid()
	val, err := r.sigarInterface.getMemResident(pid)
	assert.Equal(t, nil, err)
	assert.NotEqual(t, 0, val)
}

// Can't get proc percent with only one data point.
func TestGatherFirstProcPercentZero(t *testing.T) {
	Setup()
	duration, _ := time.ParseDuration("1s")
	interval, _ := time.ParseDuration("1s")
	r.SetSigarInterface(&FakeSigarGetter{procUsed: 2886})
	pe := &ParsedEvent{
		resourceName: "cpu_percent",
		duration:     duration,
		interval:     interval,
	}
	resourceVal, err := r.GetResource(pe, 1234)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, uint64(0), resourceVal)
}

// When we have gotten proc percent twice, then we can get the proc time.
func TestGatherProcPercent(t *testing.T) {
	Setup()
	duration, _ := time.ParseDuration("2s")
	interval, _ := time.ParseDuration("1s")
	fsg := FakeSigarGetter{procUsed: 2886}
	r.SetSigarInterface(&fsg)
	pe := &ParsedEvent{
		resourceName: "cpu_percent",
		duration:     duration,
		interval:     interval,
	}
	resourceVal, err := r.GetResource(pe, 1234)
	if err != nil {
		t.Fatal(err)
	}
	fsg.procUsed = 3849
	r.SetSigarInterface(&fsg)
	resourceVal, err = r.GetResource(pe, 1234)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEqual(t, uint64(0), resourceVal)
}

func TestParseAmountErrors(t *testing.T) {
	Setup()
	_, err := r.ParseAmount(MEMORY_USED_NAME, "2k")
	assert.Equal(t, "memory_used '2k' is not the correct format.", err.Error())

	_, err = r.ParseAmount(MEMORY_USED_NAME, "$kb")
	assert.Equal(t, "strconv.ParseUint: parsing \"$\": invalid syntax",
		err.Error())

	_, err = r.ParseAmount(MEMORY_USED_NAME, "5zb")
	assert.Equal(t, "Invalid units 'zb' on 'memory_used'.", err.Error())

	_, err = r.ParseAmount(CPU_PERCENT_NAME, "5zb")
	assert.Equal(t, "strconv.ParseUint: parsing \"5zb\": invalid syntax",
		err.Error())
}

func TestIsValidResourceName(t *testing.T) {
	Setup()
	assert.Equal(t, true, r.IsValidResourceName(MEMORY_USED_NAME))
	assert.Equal(t, true, r.IsValidResourceName(CPU_PERCENT_NAME))
	assert.Equal(t, true, r.IsValidResourceName(CPU_PERCENT_NAME))
}

func TestSaveDataReusesSlice(t *testing.T) {
	rh := &ResourceHolder{}
	rh.maxDataToStore = 3
	for i := int64(0); i < rh.maxDataToStore; i++ {
		rh.saveData(uint64(i))
	}
	data := rh.dataTimestamps
	assert.Equal(t, uint64(0), data[0].data)
	assert.Equal(t, uint64(rh.maxDataToStore-1), data[len(data)-1].data)
	rh.saveData(1337)
	assert.Equal(t, uint64(1337), data[0].data)
	rh.saveData(1338)
	assert.Equal(t, uint64(1338), data[1].data)
}

func TestCircularProcData(t *testing.T) {
	duration, _ := time.ParseDuration("3s")
	interval, _ := time.ParseDuration("1s")
	// 9 / 3 = 3 pieces of data max.
	maxData := 3
	pe := &ParsedEvent{
		resourceName: "cpu_percent",
		duration:     duration,
		interval:     interval,
	}
	for i := 0; i < maxData; i++ {
		fsg := &FakeSigarGetter{
			procUsed: uint64(2886 + i),
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
	assert.Equal(t, uint64(2886), rh.dataTimestamps[0].data)
	assert.Equal(t, uint64(2888),
		rh.dataTimestamps[len(rh.dataTimestamps)-1].data)

	// Now, make sure that we loop on the data slice.
	fsg := &FakeSigarGetter{
		procUsed: uint64(2897),
	}
	r.SetSigarInterface(fsg)
	resourceVal, err := r.GetResource(pe, 1234)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, uint64(2897), rh.dataTimestamps[0].data)
	time1 := float64(rh.dataTimestamps[0].nanoTimestamp) / NANO_TO_MILLI
	time2 := float64(rh.dataTimestamps[maxData-2].nanoTimestamp) / NANO_TO_MILLI
	expected := uint64(100 * (float64(2897-2887) / (time1 - time2)))
	assert.Equal(t, expected, resourceVal)
}
