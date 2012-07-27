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
	procUsed    float64
	sysMemUsed  uint64
}

func (s *FakeSigarGetter) getMem(pid int, kind string) (uint64, error) {
	return (s.memResident), nil
}

// Gets the proc time and a timestamp and returns a DataTimestamp.
func (s *FakeSigarGetter) getProcTime(pid int) (float64, error) {
	return float64(s.procUsed), nil
}

func (s *FakeSigarGetter) getSysMem(kind string) (uint64, error) {
	return uint64(s.sysMemUsed), nil
}

var r ResourceManager

func Setup() {
	r = ResourceManager{sigarInterface: &SigarGetter{}}
}

func TestCalculateProcPercent(t *testing.T) {
	Setup()
	first := DataTimestamp{
		data: float64(2886), nanoTimestamp: float64(1.342471447022575e+18)}
	second := DataTimestamp{
		data: float64(3849), nanoTimestamp: float64(1.342471449022077e+18)}
	var data []DataTimestamp
	data = append(data, first)
	data = append(data, second)
	resourceHolder := ResourceHolder{dataTimestamps: data}
	proc, err := resourceHolder.calculateProcPercent()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 0.48161999345784273, proc)
}

func TestGatherMem(t *testing.T) {
	Setup()
	duration, _ := time.ParseDuration("0s")
	r.SetSigarInterface(&FakeSigarGetter{memResident: 1024})
	pe := &ParsedEvent{
		resourceName: MEMORY_USED_NAME,
		duration:     duration,
	}
	resourceVal, err := r.GetResource(pe, 1234)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, uint64(1024), resourceVal)
}

func TestGetMem(t *testing.T) {
	Setup()
	pid := os.Getpid()
	memTypes := []string{
		"size", "used", "share", "minor_faults", "major_faults", "page_faults"}
	for _, memType := range memTypes {
		val, err := r.sigarInterface.getMem(pid, memType)
		assert.Equal(t, nil, err)
		assert.NotEqual(t, 0, val)
	}
	val, err := r.sigarInterface.getMem(pid, "doesntexist")
	assert.Equal(t, "Unknown resource 'memory_doesntexist'.", err.Error())
	assert.Equal(t, uint64(0), val)
}

func TestGetSysMem(t *testing.T) {
	Setup()
	memTypes := []string{"total", "used", "free", "actual_free", "actual_used"}
	for _, memType := range memTypes {
		val, err := r.sigarInterface.getSysMem(memType)
		assert.Equal(t, nil, err)
		assert.NotEqual(t, 0, val)
	}
	val, err := r.sigarInterface.getSysMem("doesntexist")
	assert.Equal(t, "Unknown resource 'sys_memory_doesntexist'.", err.Error())
	assert.Equal(t, uint64(0), val)
}

// Can't get proc percent with only one data point.
func TestGatherFirstProcPercentZero(t *testing.T) {
	Setup()
	duration, _ := time.ParseDuration("0s")
	r.SetSigarInterface(&FakeSigarGetter{procUsed: 2886})
	pe := &ParsedEvent{
		resourceName: "cpu_percent",
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
	Setup()
	duration, _ := time.ParseDuration("0s")
	fsg := FakeSigarGetter{procUsed: 2886}
	r.SetSigarInterface(&fsg)
	pe := &ParsedEvent{
		resourceName: "cpu_percent",
		duration:     duration,
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
	assert.NotEqual(t, float64(0), resourceVal)
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
	assert.Equal(t, "strconv.ParseFloat: parsing \"5zb\": invalid syntax",
		err.Error())
}

func TestIsValidResourceName(t *testing.T) {
	Setup()
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
	data := rh.dataTimestamps
	assert.Equal(t, 0, data[0].data)
	assert.Equal(t, MAX_DATA_TO_STORE-1, data[len(data)-1].data)
	rh.saveData(1337)
	assert.Equal(t, 1337, data[0].data)
	rh.saveData(1338)
	assert.Equal(t, 1338, data[1].data)
	MAX_DATA_TO_STORE = oldMax
}

func TestCircularProcData(t *testing.T) {
	oldMax := MAX_DATA_TO_STORE
	MAX_DATA_TO_STORE = 3
	pe := &ParsedEvent{
		resourceName: "cpu_percent",
	}
	for i := 0; i < MAX_DATA_TO_STORE; i++ {
		fsg := &FakeSigarGetter{
			procUsed: float64(2886 + i),
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
	assert.Equal(t, float64(2886), rh.dataTimestamps[0].data)
	assert.Equal(t, float64(2888),
		rh.dataTimestamps[len(rh.dataTimestamps)-1].data)

	// Now, make sure that we loop on the data slice.
	fsg := &FakeSigarGetter{
		procUsed: float64(2897),
	}
	r.SetSigarInterface(fsg)
	resourceVal, err := r.GetResource(pe, 1234)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, float64(2897), rh.dataTimestamps[0].data)
	time1 := rh.dataTimestamps[0].nanoTimestamp / NANO_TO_MILLI
	time2 := rh.dataTimestamps[MAX_DATA_TO_STORE-1].nanoTimestamp / NANO_TO_MILLI
	expected := float64(2897-2888) / (time1 - time2)
	assert.Equal(t, expected, resourceVal)
	MAX_DATA_TO_STORE = oldMax
}
