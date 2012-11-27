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

	helper.CreateProcessGroupCfg("vcap", s.dir,
		&gonit.ProcessGroup{Processes: s.procs})

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

func (s *DependsIntSuite) TestRestartGroup(c *C) {
	err := helper.RunGonitCmd("-g start vcap", s.dir)
	c.Assert(err, IsNil)

	before, err := s.statusGroup("vcap")
	c.Assert(err, IsNil)
	c.Assert(before.Group, HasLen, len(s.procs))
	for _, bg := range before.Group {
		current := bg.Summary
		comment := Commentf("process: %s", current.Name)
		c.Check(current.Running, Equals, true, comment)
	}

	err = helper.RunGonitCmd("-g restart vcap", s.dir)
	c.Assert(err, IsNil)

	after, err := s.statusGroup("vcap")
	c.Assert(err, IsNil)

	for i, bg := range before.Group {
		previous := bg.Summary
		current := after.Group[i].Summary
		comment := Commentf("process: %s", current.Name)
		c.Assert(previous.Name, Equals, current.Name)
		c.Check(current.Running, Equals, true, comment)
		c.Check(current.ControlState.Starts, Equals, previous.ControlState.Starts+1,
			comment)

	}
}

func (s *DependsIntSuite) statusGroup(group string) (*gonit.ProcessGroupStatus, error) {
	pgs := &gonit.ProcessGroupStatus{}
	err := s.client.Call("API.StatusGroup", group, pgs)
	return pgs, err
}
