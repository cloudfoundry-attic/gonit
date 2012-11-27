// Copyright (c) 2012 VMware, Inc.

package gonit_test

import (
	"fmt"
	. "github.com/cloudfoundry/gonit"
	"github.com/cloudfoundry/gonit/test/helper"
	. "launchpad.net/gocheck"
	"os"
)

type ControlSuite struct{}

var _ = Suite(&ControlSuite{})

var (
	groupName        = "controlTest"
	gonitPersistFile = ".gonit.persist.yml"
)

type FakeEventMonitor struct {
	numStartMonitoringCalled int
}

func (fem *FakeEventMonitor) StartMonitoringProcess(process *Process) {
	fem.numStartMonitoringCalled++
}
func (fem *FakeEventMonitor) Start(configManager *ConfigManager,
	control *Control) error {
	return nil
}

func (s *ControlSuite) TearDownTest(c *C) {
	os.Remove(gonitPersistFile)
}

func (fem *FakeEventMonitor) Stop() {}

func (s *ControlSuite) TestActions(c *C) {
	fem := &FakeEventMonitor{}
	configManager := &ConfigManager{
		Settings: &Settings{PersistFile: gonitPersistFile},
	}
	ctl := &Control{ConfigManager: configManager, EventMonitor: fem}

	name := "simple"
	process := helper.NewTestProcess(name, nil, false)
	defer helper.Cleanup(process)

	err := ctl.Config().AddProcess(groupName, process)
	c.Check(err, IsNil)
	c.Check(MONITOR_NOT, Equals, ctl.State(process).Monitor)
	c.Check(0, Equals, ctl.State(process).Starts)

	rv := ctl.DoAction(name, NewControlAction(ACTION_START))
	c.Check(1, Equals, fem.numStartMonitoringCalled)
	c.Check(rv, IsNil)

	c.Check(MONITOR_INIT, Equals, ctl.State(process).Monitor)
	c.Check(1, Equals, ctl.State(process).Starts)

	c.Check(true, Equals, process.IsRunning())

	rv = ctl.DoAction(name, NewControlAction(ACTION_RESTART))
	c.Check(2, Equals, fem.numStartMonitoringCalled)
	c.Check(rv, IsNil)

	c.Check(2, Equals, ctl.State(process).Starts)

	rv = ctl.DoAction(name, NewControlAction(ACTION_STOP))
	c.Check(rv, IsNil)

	c.Check(MONITOR_NOT, Equals, ctl.State(process).Monitor)

	rv = ctl.DoAction(name, NewControlAction(ACTION_MONITOR))
	c.Check(3, Equals, fem.numStartMonitoringCalled)
	c.Check(rv, IsNil)

	c.Check(MONITOR_INIT, Equals, ctl.State(process).Monitor)
}

func (s *ControlSuite) TestDepends(c *C) {
	configManager := &ConfigManager{
		Settings: &Settings{PersistFile: gonitPersistFile},
	}
	ctl := &Control{ConfigManager: configManager, EventMonitor: &FakeEventMonitor{}}

	name := "depsimple"
	process := helper.NewTestProcess(name, nil, false)
	defer helper.Cleanup(process)

	nprocesses := 4
	var oprocesses []string

	for i := 0; i < nprocesses; i++ {
		dname := fmt.Sprintf("%s_dep_%d", name, i)
		dprocess := helper.NewTestProcess(dname, nil, false)
		defer helper.Cleanup(dprocess)

		err := ctl.Config().AddProcess(groupName, dprocess)
		c.Check(err, IsNil)
		if i%2 == 0 {
			process.DependsOn = append(process.DependsOn, dname)
		} else {
			oprocesses = append(oprocesses, dname)
		}
	}

	err := ctl.Config().AddProcess(groupName, process)
	c.Check(err, IsNil)

	// start main process
	rv := ctl.DoAction(name, NewControlAction(ACTION_START))
	c.Check(rv, IsNil)

	c.Check(true, Equals, process.IsRunning())

	// stop main process
	rv = ctl.DoAction(name, NewControlAction(ACTION_STOP))
	c.Check(rv, IsNil)
	c.Check(false, Equals, process.IsRunning())

	// save pids to verify deps are not restarted
	var dpids = make([]int, len(process.DependsOn))

	// dependencies should still be running
	for i, dname := range process.DependsOn {
		dprocess, _ := ctl.Config().FindProcess(dname)
		c.Check(true, Equals, dprocess.IsRunning())
		dpids[i], err = dprocess.Pid()
		c.Check(err, IsNil)
	}

	// check start count for main process and deps

	c.Check(1, Equals, ctl.State(process).Starts)

	for _, dname := range process.DependsOn {
		dprocess, _ := ctl.Config().FindProcess(dname)
		c.Check(dprocess, NotNil)
		c.Check(1, Equals, ctl.State(dprocess).Starts)
	}

	// other processes should not have been started
	for _, oname := range oprocesses {
		oprocess, _ := ctl.Config().FindProcess(oname)
		c.Check(oprocess, NotNil)
		c.Check(0, Equals, ctl.State(oprocess).Starts)
	}

	// test start/stop of dependant

	// start main sevice
	rv = ctl.DoAction(name, NewControlAction(ACTION_START))
	c.Check(rv, IsNil)
	c.Check(true, Equals, process.IsRunning())
	c.Check(2, Equals, ctl.State(process).Starts)

	// dependencies should still be running w/ same pids
	for i, dname := range process.DependsOn {
		dprocess, _ := ctl.Config().FindProcess(dname)
		c.Check(true, Equals, dprocess.IsRunning())
		pid, err := dprocess.Pid()
		c.Check(err, IsNil)
		c.Check(dpids[i], Equals, pid)
	}

	// stop a dependency
	rv = ctl.DoAction(process.DependsOn[0], NewControlAction(ACTION_STOP))
	c.Check(rv, IsNil)

	// dependent will also stop
	c.Check(false, Equals, process.IsRunning())

	// start a dependency
	rv = ctl.DoAction(process.DependsOn[0], NewControlAction(ACTION_START))
	c.Check(rv, IsNil)

	// main process will come back up
	c.Check(true, Equals, process.IsRunning())

	ctl.DoAction(process.Name, NewControlAction(ACTION_STOP))

	c.Check(3, Equals, ctl.State(process).Starts)

	c.Check(MONITOR_NOT, Equals, ctl.State(process).Monitor)

	// stop all dependencies
	for _, dname := range process.DependsOn {
		ctl.DoAction(dname, NewControlAction(ACTION_STOP))
	}

	// verify every process has been stopped
	ctl.Config().VisitProcesses(func(p *Process) bool {
		c.Check(false, Equals, p.IsRunning())
		return true
	})
}

func (s *ControlSuite) TestLoadPersistState(c *C) {
	configManager := &ConfigManager{
		Settings: &Settings{PersistFile: gonitPersistFile},
	}
	control := &Control{ConfigManager: configManager}
	testPersistFile := os.Getenv("PWD") + "/test/config/expected_persist_file.yml"
	process := &Process{Name: "MyProcess"}
	processes := map[string]*Process{}
	processes["MyProcess"] = process
	pgs := map[string]*ProcessGroup{}
	pgs["somegroup"] = &ProcessGroup{Processes: processes}
	configManager.ProcessGroups = pgs
	configManager.Settings.PersistFile = testPersistFile
	control.LoadPersistState()
	c.Check(control.States["MyProcess"], NotNil)
	c.Check(2, Equals, control.States["MyProcess"].Starts)
	c.Check(2, Equals, control.States["MyProcess"].Monitor)
}

func (s *ControlSuite) TestPersistData(c *C) {
	configManager := &ConfigManager{Settings: &Settings{}}
	control := &Control{ConfigManager: configManager}
	testPersistFile := os.Getenv("PWD") + "/test/config/test_persist_file.yml"
	defer os.Remove(testPersistFile)
	process := &Process{Name: "MyProcess"}
	processes := map[string]*Process{}
	processes["MyProcess"] = process
	pgs := map[string]*ProcessGroup{}
	pgs["somegroup"] = &ProcessGroup{Processes: processes}
	configManager.ProcessGroups = pgs
	configManager.Settings.PersistFile = testPersistFile
	control.LoadPersistState()
	c.Check(control.States, IsNil)
	processState := &ProcessState{Monitor: 0x2, Starts: 3}
	states := map[string]*ProcessState{}
	states["MyProcess"] = processState
	err := control.PersistStates(states)
	c.Check(err, IsNil)
	err = control.LoadPersistState()
	c.Check(err, IsNil)
	c.Check(control.States["MyProcess"], NotNil)
	c.Check(3, Equals, control.States["MyProcess"].Starts)
	c.Check(2, Equals, control.States["MyProcess"].Monitor)
}
