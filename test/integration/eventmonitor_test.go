// Copyright (c) 2012 VMware, Inc.

package gonit_integration

import (
	"github.com/cloudfoundry/gonit/test/helper"
	"io"
	. "launchpad.net/gocheck"
	"os"
	"os/exec"
)

type EventIntSuite struct {
	gonitCmd    *exec.Cmd
	stdout      io.ReadCloser
	eventCfgDir string
}

var _ = Suite(&EventIntSuite{})

var balloonPidFile string

func (s *EventIntSuite) SetUpTest(c *C) {
	s.eventCfgDir = integrationDir + "/tmp/process_tmp"
	if err := os.MkdirAll(s.eventCfgDir, 0755); err != nil {
		c.Errorf(err.Error())
	}
	balloonPidFile = s.eventCfgDir + "/balloonmem0.pid"
	err := helper.CreateGonitCfg(1, "balloonmem", "./process_tmp", "./goprocess",
		true)
	if err != nil {
		c.Errorf(err.Error())
	}
	helper.CreateGonitSettings("./gonit.pid", "./", "./process_tmp")
	s.gonitCmd, s.stdout, err = helper.StartGonit(s.eventCfgDir)
	if err != nil {
		c.Errorf(err.Error())
	}
}

func (s *EventIntSuite) TearDownTest(c *C) {
	if err := helper.StopGonit(s.gonitCmd, s.eventCfgDir); err != nil {
		c.Errorf(err.Error())
	}
	if err := os.RemoveAll(s.eventCfgDir); err != nil {
		c.Errorf(err.Error())
	}
}

func (s *EventIntSuite) TestAlertRule(c *C) {
	if err := helper.RunGonitCmd("start all", s.eventCfgDir); err != nil {
		c.Errorf(err.Error())
	}
	c.Check(true, Equals, helper.FindLogLine(s.stdout,
		"'balloonmem0' triggered 'memory_used > 1mb' for '1s'", "5s"))
}

func (s *EventIntSuite) TestRestartRule(c *C) {
	if err := helper.RunGonitCmd("start all", s.eventCfgDir); err != nil {
		c.Errorf(err.Error())
	}

	balloonPid, err := helper.ProxyReadPidFile(balloonPidFile)
	c.Check(err, IsNil)
	c.Check(helper.DoesProcessExist(balloonPid), Equals, true)
	pid1, _ := helper.ProxyReadPidFile(balloonPidFile)
	c.Check(true, Equals,
		helper.FindLogLine(s.stdout, "Executing 'restart'", "20s"))
	c.Check(true, Equals,
		helper.FindLogLine(s.stdout, "process \"balloonmem0\" started", "10s"))
	balloonPid, err = helper.ProxyReadPidFile(balloonPidFile)
	c.Check(err, IsNil)
	c.Check(helper.DoesProcessExist(balloonPid), Equals, true)
	pid2, err := helper.ProxyReadPidFile(balloonPidFile)
	if err != nil {
		c.Errorf(err.Error())
	}
	c.Check(pid1, Not(Equals), pid2)
}
