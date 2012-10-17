// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	. "launchpad.net/gocheck"
	"os"
	"time"
)

type ResourceSuite struct{}

var _ = Suite(&ResourceSuite{})

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
	r = ResourceManager{
		sigarInterface:  &SigarGetter{},
		cachedResources: map[string]uint64{},
	}
}

func (s *ResourceSuite) TestCalculateProcPercent(c *C) {
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
		c.Fatal(err)
	}
	c.Check(uint64(48), Equals, proc)
}

func (s *ResourceSuite) TestGatherMem(c *C) {
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
		c.Fatal(err)
	}
	c.Check(uint64(1024), Equals, resourceVal)
}

func (s *ResourceSuite) TestgetMemResident(c *C) {
	Setup()
	pid := os.Getpid()
	val, err := r.sigarInterface.getMemResident(pid)
	c.Check(err, IsNil)
	c.Check(0, Not(Equals), val)
}

// When we have gotten proc percent twice, then we can get the proc time.
func (s *ResourceSuite) TestGatherProcPercent(c *C) {
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
		c.Fatal(err)
	}
	fsg.procUsed = 3849
	r.SetSigarInterface(&fsg)
	timeThen := r.resourceHolders[0].dataTimestamps[0].nanoTimestamp
	// Set the time to be 1 second ago.
	r.resourceHolders[0].dataTimestamps[0].nanoTimestamp = timeThen - 1000000000
	r.ClearCachedResources()
	resourceVal, err = r.GetResource(pe, 1234)
	if err != nil {
		c.Fatal(err)
	}
	c.Check(uint64(0), Not(Equals), resourceVal)
}

func (s *ResourceSuite) TestParseAmountErrors(c *C) {
	Setup()
	_, err := r.ParseAmount(MEMORY_USED_NAME, "2k")
	c.Check("memory_used '2k' is not the correct format.", Equals, err.Error())

	_, err = r.ParseAmount(MEMORY_USED_NAME, "$kb")
	c.Check("strconv.ParseUint: parsing \"$\": invalid syntax", Equals,
		err.Error())

	_, err = r.ParseAmount(MEMORY_USED_NAME, "5zb")
	c.Check("Invalid units 'zb' on 'memory_used'.", Equals, err.Error())

	_, err = r.ParseAmount(CPU_PERCENT_NAME, "5zb")
	c.Check("strconv.ParseUint: parsing \"5zb\": invalid syntax", Equals,
		err.Error())

}

func (s *ResourceSuite) TestIsValidResourceName(c *C) {
	Setup()
	c.Check(true, Equals, r.IsValidResourceName(MEMORY_USED_NAME))
	c.Check(true, Equals, r.IsValidResourceName(CPU_PERCENT_NAME))
	c.Check(true, Equals, r.IsValidResourceName(CPU_PERCENT_NAME))
}

func (s *ResourceSuite) TestSaveDataReusesSlice(c *C) {
	rh := &ResourceHolder{}
	rh.maxDataToStore = 3
	for i := int64(0); i < rh.maxDataToStore; i++ {
		rh.saveData(uint64(i))
	}
	data := rh.dataTimestamps
	c.Check(uint64(0), Equals, data[0].data)
	c.Check(uint64(rh.maxDataToStore-1), Equals, data[len(data)-1].data)
	rh.saveData(1337)
	c.Check(uint64(1337), Equals, data[0].data)
	rh.saveData(1338)
	c.Check(uint64(1338), Equals, data[1].data)
}

func (s *ResourceSuite) TestCircularProcData(c *C) {
	Setup()
	duration, _ := time.ParseDuration("3s")
	interval, _ := time.ParseDuration("1s")
	// 3 / 1 = 3 pieces of data max.
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
		r.ClearCachedResources()
		_, err := r.GetResource(pe, 1234)
		if err != nil {
			c.Fatal(err)
		}
	}
	rh := r.getResourceHolder(pe)

	// Assert we have what we expect.
	c.Check(uint64(2886), Equals, rh.dataTimestamps[0].data)
	c.Check(uint64(2888), Equals,
		rh.dataTimestamps[len(rh.dataTimestamps)-1].data)

	// Now, make sure that we loop on the data slice.
	fsg := &FakeSigarGetter{
		procUsed: uint64(2897),
	}
	r.SetSigarInterface(fsg)
	r.ClearCachedResources()
	_, err := r.GetResource(pe, 1234)
	if err != nil {
		c.Fatal(err)
	}
	// Assert that we looped.
	c.Check(uint64(2897), Equals, rh.dataTimestamps[0].data)
}

func (s *ResourceSuite) TestDuration(c *C) {
	Setup()
	duration, _ := time.ParseDuration("3s")
	interval, _ := time.ParseDuration("1s")
	pe := &ParsedEvent{
		resourceName: "cpu_percent",
		duration:     duration,
		interval:     interval,
	}
	// Under 2 so we can manually add the 3rd piece.
	for i := 0; i < 2; i++ {
		fsg := &FakeSigarGetter{
			procUsed: uint64(2886 + i),
		}
		r.SetSigarInterface(fsg)
		r.ClearCachedResources()
		_, err := r.GetResource(pe, 1234)
		if err != nil {
			c.Fatal(err)
		}
	}
	rh := r.getResourceHolder(pe)
	rh.dataTimestamps[0].nanoTimestamp =
		rh.dataTimestamps[0].nanoTimestamp - 2000000000
	rh.dataTimestamps[1].nanoTimestamp =
		rh.dataTimestamps[1].nanoTimestamp - 1000000000

	fsg := &FakeSigarGetter{
		procUsed: uint64(4000),
	}
	r.SetSigarInterface(fsg)
	r.ClearCachedResources()
	resourceVal, err := r.GetResource(pe, 1234)
	if err != nil {
		c.Fatal(err)
	}
	timeDiff := float64(rh.dataTimestamps[2].nanoTimestamp-
		rh.dataTimestamps[0].nanoTimestamp) / NANO_TO_MILLI
	valDiff := rh.dataTimestamps[2].data - rh.dataTimestamps[0].data
	expectedPercent := uint64(100 * (float64(valDiff) / float64(timeDiff)))
	c.Check(expectedPercent, Equals, resourceVal)
}

func (s *ResourceSuite) TestCleanProcData(c *C) {
	Setup()
	dts := &DataTimestamp{
		data:          1,
		nanoTimestamp: 1,
	}
	rh := &ResourceHolder{
		dataTimestamps:  []*DataTimestamp{dts},
		firstEntryIndex: 1,
		processName:     "process",
		resourceName:    "resource",
	}
	proc := &Process{Name: "process"}
	r.resourceHolders = append(r.resourceHolders, rh)
	c.Check(1, Equals, len(r.resourceHolders[0].dataTimestamps))
	c.Check(int64(1), Equals, r.resourceHolders[0].firstEntryIndex)
	r.CleanDataForProcess(proc)
	c.Check(0, Equals, len(r.resourceHolders[0].dataTimestamps))
	c.Check(int64(0), Equals, r.resourceHolders[0].firstEntryIndex)
}
