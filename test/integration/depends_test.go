// Copyright (c) 2012 VMware, Inc.

package gonit_integration

import (
	"fmt"
	"github.com/cloudfoundry/gonit"
	"github.com/cloudfoundry/gonit/test/helper"
	"io"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os/exec"
)

type DependsIntSuite struct {
	gonitCmd *exec.Cmd
	dir      string
	procs    map[string]*gonit.Process
	settings *gonit.Settings
	client   *rpc.Client
}

var _ = Suite(&DependsIntSuite{})

func (s *DependsIntSuite) addProcess(name string) *gonit.Process {
	flags := []string{"-s", "1h"}
	process := helper.NewTestProcess(name, flags, true)
	process.Description = name
	s.procs[process.Name] = process
	process.MonitorMode = gonit.MONITOR_MODE_MANUAL
	return process
}

func (s *DependsIntSuite) SetUpSuite(c *C) {
	s.procs = make(map[string]*gonit.Process)
	s.addProcess("nginx")
	s.addProcess("postgres")
	s.addProcess("redis")
	s.addProcess("nats")

	s.addProcess("director").DependsOn = []string{"postgres", "redis", "nats"}

	for i := 0; i < 3; i++ {
		worker := fmt.Sprintf("worker_%d", i)
		s.addProcess(worker).DependsOn = []string{"director"}
	}

	s.addProcess("aws_registry").DependsOn = []string{"postgres"}

	s.dir = c.MkDir()

	s.settings = helper.CreateGonitSettings("", s.dir, s.dir)

	helper.CreateProcessGroupCfg("bosh", s.dir, &gonit.ProcessGroup{Processes: s.procs})

	vcap := make(map[string]*gonit.Process)
	for _, name := range []string{"dea", "router", "stager"} {
		vcap[name] = s.addProcess(name)
	}

	helper.CreateProcessGroupCfg("vcap", s.dir, &gonit.ProcessGroup{Processes: vcap})

	var err error
	var stdout io.Reader
	s.gonitCmd, stdout, err = helper.StartGonit(s.dir)
	// TODO: log Writes block when the pipe buffer is full,
	// so we must drain the pipe here
	go io.Copy(ioutil.Discard, stdout)

	if err != nil {
		c.Errorf(err.Error())
	}

	s.client, err = jsonrpc.Dial("unix", s.settings.RpcServerUrl)
	if err != nil {
		c.Errorf("rpc.Dial: %v", err)
	}
}

func (s *DependsIntSuite) TearDownSuite(c *C) {
	if s.client != nil {
		s.client.Close()
	}

	if err := helper.StopGonit(s.gonitCmd, s.dir); err != nil {
		c.Errorf(err.Error())
	}

	for _, process := range s.procs {
		helper.Cleanup(process)
	}
}

func (s *DependsIntSuite) TestRestartAll(c *C) {
	var err error
	all := s.newGroupHelper("all")

	// start all processes
	err = all.runCmd("start")
	c.Assert(err, IsNil)

	err = all.status(all.before)
	c.Assert(err, IsNil)

	// check all processes are running
	for _, bg := range all.before {
		current := bg.Summary
		comment := Commentf("process: %s", current.Name)
		c.Check(current.Running, Equals, true, comment)
	}

	// restart all processes
	err = all.runCmd("restart")
	c.Assert(err, IsNil)

	err = all.status(all.after)
	c.Assert(err, IsNil)

	// each process should only restart once
	all.compare(func(previous, current *gonit.ProcessSummary) {
		comment := Commentf("process: %s", current.Name)
		c.Assert(previous.Name, Equals, current.Name)
		c.Check(current.Running, Equals, true, comment)
		c.Check(current.ControlState.Starts, Equals, previous.ControlState.Starts+1, comment)
	})
}

func (s *DependsIntSuite) TestRestartGroup(c *C) {
	var err error
	bosh := s.newGroupHelper("bosh")
	vcap := s.newGroupHelper("vcap")

	// start bosh and vcap groups
	for _, group := range []*groupHelper{bosh, vcap} {
		err = group.runCmd("start")
		c.Assert(err, IsNil)

		err = group.status(group.before)
		c.Assert(err, IsNil)

		// check all processes are running
		for _, bg := range group.before {
			current := bg.Summary
			comment := Commentf("process: %s", current.Name)
			c.Check(current.Running, Equals, true, comment)
		}
	}

	// restart bosh group
	err = bosh.runCmd("restart")
	c.Assert(err, IsNil)

	err = bosh.status(bosh.after)
	c.Assert(err, IsNil)

	// each process should only restart once
	bosh.compare(func(previous, current *gonit.ProcessSummary) {
		comment := Commentf("process: %s", current.Name)
		c.Assert(previous.Name, Equals, current.Name)
		c.Check(current.Running, Equals, true, comment)
		c.Check(current.ControlState.Starts, Equals, previous.ControlState.Starts+1, comment)
	})

	// vcap processes should not restart
	err = vcap.status(vcap.after)
	c.Assert(err, IsNil)
	vcap.compare(func(previous, current *gonit.ProcessSummary) {
		comment := Commentf("process: %s", current.Name)
		c.Assert(previous.Name, Equals, current.Name)
		c.Check(current.Running, Equals, true, comment)
		c.Check(current.ControlState.Starts, Equals, previous.ControlState.Starts, comment)
	})
}

func (s *DependsIntSuite) TestProcessDependencies(c *C) {
	var err error
	bosh := s.newGroupHelper("bosh")
	process := s.procs["director"]

	// stop all bosh processes
	err = bosh.runCmd("stop")
	c.Assert(err, IsNil)

	err = bosh.status(bosh.before)
	c.Assert(err, IsNil)

	// check all processes are stopped
	for _, bg := range bosh.before {
		current := bg.Summary
		comment := Commentf("process: %s", current.Name)
		c.Check(current.Running, Equals, false, comment)
	}

	// start director process
	err = helper.RunGonitCmd("start director", s.dir)
	c.Assert(err, IsNil)

	err = bosh.status(bosh.after)
	c.Assert(err, IsNil)

	bosh.compare(func(previous, current *gonit.ProcessSummary) {
		running := false
		delta := 0
		// only director and its dependencies should start
		if current.Name == process.Name || dependsOn(process, current.Name) {
			running = true
			delta = 1
		}
		comment := Commentf("process: %s", current.Name)
		c.Assert(previous.Name, Equals, current.Name)
		c.Check(current.Running, Equals, running, comment)
		c.Check(current.ControlState.Starts, Equals, previous.ControlState.Starts+delta, comment)
	})

	bosh.before = bosh.after
	// restart a director dependency
	err = helper.RunGonitCmd("restart "+process.DependsOn[0], s.dir)
	c.Assert(err, IsNil)

	err = bosh.status(bosh.after)
	c.Assert(err, IsNil)

	bosh.compare(func(previous, current *gonit.ProcessSummary) {
		running := false
		delta := 0
		// only director and its dependencies should be running
		if current.Name == process.Name || dependsOn(process, current.Name) {
			running = true
		}
		// only director and restarted dependency should restart
		if current.Name == process.Name || current.Name == process.DependsOn[0] {
			delta = 1
		}

		comment := Commentf("process: %s", current.Name)
		c.Assert(previous.Name, Equals, current.Name)
		c.Check(current.Running, Equals, running, comment)
		c.Check(current.ControlState.Starts, Equals, previous.ControlState.Starts+delta, comment)
	})

	bosh.before = bosh.after
	// stop director process
	err = helper.RunGonitCmd("stop director", s.dir)
	c.Assert(err, IsNil)

	err = bosh.status(bosh.after)
	c.Assert(err, IsNil)

	bosh.compare(func(previous, current *gonit.ProcessSummary) {
		running := false
		// only director dependencies should still be running
		if dependsOn(process, current.Name) {
			running = true
		}
		comment := Commentf("process: %s", current.Name)
		c.Assert(previous.Name, Equals, current.Name)
		c.Check(current.Running, Equals, running, comment)
		c.Check(current.ControlState.Starts, Equals, previous.ControlState.Starts, comment)
	})
}

func dependsOn(process *gonit.Process, name string) bool {
	for _, dep := range process.DependsOn {
		if dep == name {
			return true
		}
	}
	return false
}

type groupHelper struct {
	name          string
	suite         *DependsIntSuite
	before, after []gonit.ProcessStatus
}

func (d *groupHelper) compare(f func(previous, current *gonit.ProcessSummary)) {
	for i := 0; i < len(d.before); i++ {
		f(&d.before[i].Summary, &d.after[i].Summary)
	}
}

func (d *groupHelper) isAll() bool {
	return d.name == "all"
}

func (d *groupHelper) runCmd(name string) error {
	cmd := ""
	if !d.isAll() {
		cmd += "-g "
	}
	cmd += name + " " + d.name
	return helper.RunGonitCmd(cmd, d.suite.dir)
}

func (d *groupHelper) status(ps []gonit.ProcessStatus) error {
	var method string
	if d.isAll() {
		method = "StatusAll"
	} else {
		method = "StatusGroup"
	}
	pgs := &gonit.ProcessGroupStatus{}
	err := d.suite.client.Call("API."+method, d.name, pgs)
	if err != nil {
		copy(ps, pgs.Group)
	}
	return err
}

func (s *DependsIntSuite) newGroupHelper(name string) *groupHelper {
	return &groupHelper{
		name:  name,
		suite: s,
	}
}
