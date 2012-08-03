// Copyright (c) 2012 VMware, Inc.

package gonit_test

import (
	"fmt"
	"github.com/bmizerany/assert"
	. "github.com/cloudfoundry/gonit"
	"github.com/cloudfoundry/gonit/test/helper"
	"testing"
)

var groupName = "controlTest"

func TestActions(t *testing.T) {
	c := &Control{}

	name := "simple"
	process := helper.NewTestProcess(name, nil, false)
	defer helper.Cleanup(process)

	err := c.Config().AddProcess(groupName, process)
	assert.Equal(t, nil, err)

	assert.Equal(t, MONITOR_NOT, c.State(process).Monitor)
	assert.Equal(t, 0, c.State(process).Starts)

	rv := c.DoAction(name, ACTION_START)
	assert.Equal(t, true, rv)

	assert.Equal(t, MONITOR_INIT, c.State(process).Monitor)
	assert.Equal(t, 1, c.State(process).Starts)

	assert.Equal(t, true, process.IsRunning())

	rv = c.DoAction(name, ACTION_RESTART)
	assert.Equal(t, true, rv)

	assert.Equal(t, 2, c.State(process).Starts)

	rv = c.DoAction(name, ACTION_STOP)
	assert.Equal(t, true, rv)

	assert.Equal(t, MONITOR_NOT, c.State(process).Monitor)

	rv = c.DoAction(name, ACTION_MONITOR)
	assert.Equal(t, true, rv)

	assert.Equal(t, MONITOR_INIT, c.State(process).Monitor)
}

func TestDepends(t *testing.T) {
	c := &Control{}

	name := "depsimple"
	process := helper.NewTestProcess(name, nil, false)
	defer helper.Cleanup(process)

	nprocesses := 4
	var oprocesses []string

	for i := 0; i < nprocesses; i++ {
		dname := fmt.Sprintf("%s_dep_%d", name, i)
		dprocess := helper.NewTestProcess(dname, nil, false)
		defer helper.Cleanup(dprocess)

		err := c.Config().AddProcess(groupName, dprocess)
		assert.Equal(t, nil, err)
		if i%2 == 0 {
			process.DependsOn = append(process.DependsOn, dname)
		} else {
			oprocesses = append(oprocesses, dname)
		}
	}

	err := c.Config().AddProcess(groupName, process)
	assert.Equal(t, nil, err)

	// start main process
	rv := c.DoAction(name, ACTION_START)
	assert.Equal(t, true, rv)

	assert.Equal(t, true, process.IsRunning())

	// stop main process
	rv = c.DoAction(name, ACTION_STOP)
	assert.Equal(t, true, rv)
	assert.Equal(t, false, process.IsRunning())

	// save pids to verify deps are not restarted
	var dpids = make([]int, len(process.DependsOn))

	// dependencies should still be running
	for i, dname := range process.DependsOn {
		dprocess, _ := c.Config().FindProcess(dname)
		assert.Equal(t, true, dprocess.IsRunning())
		dpids[i], err = dprocess.Pid()
		assert.Equal(t, nil, err)
	}

	// check start count for main process and deps

	assert.Equal(t, 1, c.State(process).Starts)

	for _, dname := range process.DependsOn {
		dprocess, _ := c.Config().FindProcess(dname)
		assert.NotEqual(t, nil, dprocess)
		assert.Equal(t, 1, c.State(dprocess).Starts)
	}

	// other processes should not have been started
	for _, oname := range oprocesses {
		oprocess, _ := c.Config().FindProcess(oname)
		assert.NotEqual(t, nil, oprocess)
		assert.Equal(t, 0, c.State(oprocess).Starts)
	}

	// test start/stop of dependant

	// start main sevice
	rv = c.DoAction(name, ACTION_START)
	assert.Equal(t, true, rv)
	assert.Equal(t, true, process.IsRunning())
	assert.Equal(t, 2, c.State(process).Starts)

	// dependencies should still be running w/ same pids
	for i, dname := range process.DependsOn {
		dprocess, _ := c.Config().FindProcess(dname)
		assert.Equal(t, true, dprocess.IsRunning())
		pid, err := dprocess.Pid()
		assert.Equal(t, nil, err)
		assert.Equal(t, dpids[i], pid)
	}

	// stop a dependency
	rv = c.DoAction(process.DependsOn[0], ACTION_STOP)
	assert.Equal(t, true, rv)

	// dependent will also stop
	assert.Equal(t, false, process.IsRunning())

	// start a dependency
	rv = c.DoAction(process.DependsOn[0], ACTION_START)
	assert.Equal(t, true, rv)

	// main process will come back up
	assert.Equal(t, true, process.IsRunning())

	c.DoAction(process.Name, ACTION_STOP)

	assert.Equal(t, 3, c.State(process).Starts)

	assert.Equal(t, MONITOR_NOT, c.State(process).Monitor)

	// stop all dependencies
	for _, dname := range process.DependsOn {
		c.DoAction(dname, ACTION_STOP)
	}

	// verify every process has been stopped
	c.Config().VisitProcesses(func(p *Process) bool {
		assert.Equal(t, false, p.IsRunning())
		return true
	})
}
