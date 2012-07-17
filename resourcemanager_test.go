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
	timeNow     int64
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
		nanoTimestamp: float64(s.timeNow),
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
	assert.Equal(t, 0.48161999345784273, resourceHolder.calculateProcPercent())
}

func TestGatherMem(t *testing.T) {
	interval, _ := time.ParseDuration("1s")
	duration, _ := time.ParseDuration("0s")
	r.SetSigarInterface(&FakeSigarGetter{memResident: 1024})
	pe := ParsedEvent{
		resourceName: MEMORY_USED_NAME,
		interval:     interval,
		duration:     duration,
	}
	resourceVal, err := r.GetResource(&pe, 1234)
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
	pe := ParsedEvent{
		resourceName: "cpu_percent",
		interval:     interval,
		duration:     duration,
	}
	resourceVal, err := r.GetResource(&pe, 1234)
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
	pe := ParsedEvent{
		resourceName: "cpu_percent",
		interval:     interval,
		duration:     duration,
	}
	resourceVal, err := r.GetResource(&pe, 1234)
	if err != nil {
		t.Fatal(err)
	}
	fsg.procUsed = 3849
	fsg.timeNow = 1.342471449022077e+18
	r.SetSigarInterface(&fsg)
	resourceVal, err = r.GetResource(&pe, 1234)
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
