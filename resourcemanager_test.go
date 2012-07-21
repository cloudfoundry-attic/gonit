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
	timeNow     int64
	sysMemUsed  uint64
}

// Gets the Resident memory of a process.
func (s FakeSigarGetter) getMem(pid int, kind string) (uint64, error) {
	return (s.memResident), nil
}

// Gets the proc time and a timestamp and returns a DataTimestamp.
func (s FakeSigarGetter) getProcTime(pid int) (float64, error) {
	return float64(s.procUsed), nil
}

func (s FakeSigarGetter) getSysMem(kind string) (uint64, error) {
	return uint64(s.sysMemUsed), nil
}

var r ResourceManager

func Setup() {
	r = ResourceManager{sigarInterface: SigarGetter{}}
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
	assert.Equal(t, 0.48161999345784273, calculateProcPercent(data))
}

func TestGatherMem(t *testing.T) {
	Setup()
	duration, _ := time.ParseDuration("0s")
	r.SetSigarInterface(FakeSigarGetter{memResident: 1024})
	resourceVal, err := r.GetResource(1234, MEM_USED_NAME, duration)
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
	assert.Equal(t, "Unknown resource 'mem_doesntexist'.", err.Error())
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
	assert.Equal(t, "Unknown resource 'sys_mem_doesntexist'.", err.Error())
	assert.Equal(t, uint64(0), val)
}

// Can't get proc percent with only one data point.
func TestGatherFirstProcPercentZero(t *testing.T) {
	Setup()
	duration, _ := time.ParseDuration("0s")
	r.SetSigarInterface(FakeSigarGetter{procUsed: 2886,
		timeNow: 1.342471447022575e+18})
	resourceVal, err := r.GetResource(1234, "proc_percent", duration)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, float64(0), resourceVal)
}

// When we have gotten proc percent twice, then we can get the proc time.
func TestGatherProcPercent(t *testing.T) {
	Setup()
	duration, _ := time.ParseDuration("0s")
	fsg := FakeSigarGetter{
		procUsed: float64(2886),
	}
	r.SetSigarInterface(fsg)
	resourceVal, err := r.GetResource(1234, "proc_percent", duration)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, float64(0), resourceVal)
	fsg = FakeSigarGetter{
		procUsed: float64(3849),
	}
	r.SetSigarInterface(fsg)
	resourceVal, err = r.GetResource(1234, "proc_percent", duration)

	if err != nil {
		t.Fatal(err)
	}
	assert.NotEqual(t, float64(0), resourceVal)
}

func TestParseAmountErrors(t *testing.T) {
	Setup()
	_, err := r.ParseAmount(MEM_USED_NAME, "2k")
	assert.Equal(t, "mem_used '2k' is not the correct format.", err.Error())

	_, err = r.ParseAmount(MEM_USED_NAME, "$kb")
	assert.Equal(t, "strconv.ParseUint: parsing \"$\": invalid syntax",
		err.Error())

	_, err = r.ParseAmount(MEM_USED_NAME, "5zb")
	assert.Equal(t, "Invalid units 'zb' on 'mem_used'.", err.Error())

	_, err = r.ParseAmount(PROC_PERCENT_NAME, "5zb")
	assert.Equal(t, "strconv.ParseFloat: parsing \"5zb\": invalid syntax",
		err.Error())
}

func TestIsValidResourceName(t *testing.T) {
	Setup()
	assert.Equal(t, true, r.IsValidResourceName(MEM_USED_NAME))
	assert.Equal(t, true, r.IsValidResourceName(PROC_PERCENT_NAME))
	assert.Equal(t, true, r.IsValidResourceName(PROC_PERCENT_NAME))
}
