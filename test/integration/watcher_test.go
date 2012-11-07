// Copyright (c) 2012 VMware, Inc.

package gonit_integration

import (
	"github.com/cloudfoundry/gonit"
	"github.com/cloudfoundry/gonit/test/helper"
	. "launchpad.net/gocheck"
	"os"
	"os/exec"
	"time"
)

type WatcherIntSuite struct {
	gonitCmd *exec.Cmd
	dir      string
	procs    map[string]*gonit.Process
	settings *gonit.Settings
}

var _ = Suite(&WatcherIntSuite{})

func (s *WatcherIntSuite) addProcess(name string, flags []string) *gonit.Process {
	process := helper.NewTestProcess(name, flags, false)
	process.Description = name
	s.procs[process.Name] = process
	return process
}

func (s *WatcherIntSuite) SetUpTest(c *C) {
	s.procs = make(map[string]*gonit.Process)
	s.addProcess("active", nil)
	s.addProcess("manual", nil).MonitorMode = gonit.MONITOR_MODE_MANUAL
	s.addProcess("passive", nil).MonitorMode = gonit.MONITOR_MODE_PASSIVE

	s.dir = c.MkDir()
	s.settings = helper.CreateGonitSettings("", s.dir, s.dir)
	os.Remove(s.settings.PersistFile)

	helper.CreateProcessGroupCfg("watch", s.dir,
		&gonit.ProcessGroup{Processes: s.procs})

	var err error
	s.gonitCmd, _, err = helper.StartGonit(s.dir)
	if err != nil {
		c.Errorf(err.Error())
	}
}

func (s *WatcherIntSuite) TearDownTest(c *C) {
	if err := helper.StopGonit(s.gonitCmd, s.dir); err != nil {
		c.Errorf(err.Error())
	}

	for _, process := range s.procs {
		helper.Cleanup(process)
	}
}

func (s *WatcherIntSuite) TestStart(c *C) {
	c.Check(s.procs["active"].IsRunning(), Equals, true)
	c.Check(s.procs["manual"].IsRunning(), Equals, false)
	c.Check(s.procs["passive"].IsRunning(), Equals, false)

	helper.RunGonitCmd("start all", s.dir)
	for _, process := range s.procs {
		c.Check(process.IsRunning(), Equals, true)
	}
}

func (s *WatcherIntSuite) TestRecover(c *C) {
	process := s.procs["active"]
	c.Check(process.IsRunning(), Equals, true)
	pid1, err := process.Pid()
	c.Check(err, IsNil)

	err = process.StopProcess()
	c.Check(err, IsNil)

	interval := time.Second * 10 // TODO: time.Duration(s.settings.ProcessPollInterval)
	time.Sleep(interval)
	c.Check(process.IsRunning(), Equals, true)

	pid2, err := process.Pid()
	c.Check(err, IsNil)

	c.Check(pid1, Not(Equals), pid2)
}
