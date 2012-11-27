// Copyright (c) 2012 VMware, Inc.

package gonit_integration

import (
	"fmt"
	"github.com/cloudfoundry/gonit"
	"github.com/cloudfoundry/gonit/test/helper"
	. "launchpad.net/gocheck"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os/exec"
	"time"
)

type ApiIntSuite struct {
	gonitCmd *exec.Cmd
	dir      string
	procs    map[string]*gonit.Process
	settings *gonit.Settings
	client   *rpc.Client
}

const (
	errorInProgressFmt = "ActionError: " + gonit.ERROR_IN_PROGRESS_FMT
)

var _ = Suite(&ApiIntSuite{})

func (s *ApiIntSuite) addProcess(name string, flags []string) *gonit.Process {
	process := helper.NewTestProcess(name, flags, true)
	process.Description = name
	process.MonitorMode = gonit.MONITOR_MODE_MANUAL
	s.procs[process.Name] = process
	return process
}

func (s *ApiIntSuite) SetUpSuite(c *C) {
	s.procs = make(map[string]*gonit.Process)
	s.addProcess("sleepy", []string{"-s", "1h"})
	s.addProcess("dopey", []string{"-w", "5s", "-s", "1h"})
	s.addProcess("grumpy", []string{"-x", "1", "-s", "1s"})

	s.dir = c.MkDir()

	s.settings = helper.CreateGonitSettings("", s.dir, s.dir)

	helper.CreateProcessGroupCfg("api_test", s.dir,
		&gonit.ProcessGroup{Processes: s.procs})

	var err error
	s.gonitCmd, _, err = helper.StartGonit(s.dir)
	if err != nil {
		c.Errorf(err.Error())
	}

	s.client, err = jsonrpc.Dial("unix", s.settings.RpcServerUrl)
	if err != nil {
		c.Errorf("rpc.Dial: %v", err)
	}

	pgs, err := s.statusGroup("api_test")
	c.Assert(err, IsNil)
	c.Assert(pgs.Group, HasLen, len(s.procs))
	for _, ps := range pgs.Group {
		c.Assert(ps.Summary.Running, Equals, false)
	}
}

func (s *ApiIntSuite) TearDownSuite(c *C) {
	s.client.Close()

	if err := helper.StopGonit(s.gonitCmd, s.dir); err != nil {
		c.Errorf(err.Error())
	}

	for _, process := range s.procs {
		helper.Cleanup(process)
	}
}

func (s *ApiIntSuite) TestControl(c *C) {
	dopey := s.procs["dopey"]
	grumpy := s.procs["grumpy"]
	sleepy := s.procs["sleepy"]

	result, err := s.startProcess(sleepy)
	c.Check(err, IsNil)
	c.Check(result.Total, Equals, 1)
	c.Check(result.Errors, Equals, 0)

	done := make(chan error)
	go func() {
		// takes a while to write pid file
		_, err := s.startProcess(dopey)
		done <- err
	}()

	// make sure above StartProcess is in action
	time.Sleep(2 * time.Second)

	// test we can get status while control action is running
	status, err := s.statusProcess(dopey)
	if c.Check(err, IsNil) {
		c.Check(status.Summary.Running, Equals, false)
	}

	// get status for another process should be fine too
	status, err = s.statusProcess(grumpy)
	if c.Check(err, IsNil) {
		c.Check(status.Summary.Running, Equals, false)
	}

	// control action in already progress; should fail
	_, err = s.stopProcess(dopey)
	msg := fmt.Sprintf(errorInProgressFmt, dopey.Name)
	if c.Check(err, NotNil) {
		c.Check(err.Error(), Equals, msg)
	}

	// but can control another process
	_, err = s.startProcess(grumpy)
	c.Check(err, IsNil)

	err = <-done // waiting for dopey to start
	c.Check(err, IsNil)

	status, err = s.statusProcess(dopey)
	if c.Check(err, IsNil) {
		c.Check(status.Summary.Name, Equals, dopey.Name)
		c.Check(status.Summary.Running, Equals, true)
	}

	status, err = s.statusProcess(sleepy)
	if c.Check(err, IsNil) {
		c.Check(status.Summary.Running, Equals, true)
	}
}

func (s *ApiIntSuite) statusProcess(p *gonit.Process) (*gonit.ProcessStatus, error) {
	status := &gonit.ProcessStatus{}
	err := s.client.Call("API.StatusProcess", p.Name, status)
	return status, err
}

func (s *ApiIntSuite) startProcess(p *gonit.Process) (*gonit.ActionResult, error) {
	result := &gonit.ActionResult{}
	err := s.client.Call("API.StartProcess", p.Name, result)
	return result, err
}

func (s *ApiIntSuite) stopProcess(p *gonit.Process) (*gonit.ActionResult, error) {
	result := &gonit.ActionResult{}
	err := s.client.Call("API.StopProcess", p.Name, result)
	return result, err
}

func (s *ApiIntSuite) statusGroup(group string) (*gonit.ProcessGroupStatus, error) {
	pgs := &gonit.ProcessGroupStatus{}
	err := s.client.Call("API.StatusGroup", group, pgs)
	return pgs, err
}
