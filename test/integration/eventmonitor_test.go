// Copyright (c) 2012 VMware, Inc.

package gonit_integration

import (
	"github.com/cloudfoundry/gonit/test/helper"
	"io"
	. "launchpad.net/gocheck"
	"os/exec"
)

type EventIntSuite struct {
	gonitCmd     *exec.Cmd
	expectedExit *bool
	stdout       io.ReadCloser
	eventCfgDir  string
}

var _ = Suite(&EventIntSuite{})

var balloonPidFile = "../test/integration/tmp/process_tmp/balloonmem.pid"

func (s *EventIntSuite) SetUpTest(c *C) {
	s.eventCfgDir = integrationDir + "/eventmonitor_integration_cfg"
	var err error
	s.gonitCmd, s.stdout, s.expectedExit, err =
		helper.StartGonit(s.eventCfgDir, verbose)
	if err != nil {
		c.Errorf(err.Error())
	}
}

func (s *EventIntSuite) TearDownTest(c *C) {
	if err := helper.StopGonit(s.gonitCmd, verbose, s.expectedExit,
		s.eventCfgDir); err != nil {
		c.Errorf(err.Error())
	}
	helper.RemoveFilesWithExtension(integrationDir+"/tmp/process_tmp", ".pid")
	helper.RemoveFilesWithExtension(integrationDir+"/tmp/process_tmp", ".json")
}

func (s *EventIntSuite) TestAlertRule(c *C) {
	if err := helper.RunGonitCmd("start all", verbose,
		s.eventCfgDir); err != nil {
		c.Errorf(err.Error())
	}
	c.Check(true, Equals, helper.FindLogLine(s.stdout,
		"'balloonmem' triggered 'memory_used > 1mb' for '1s'", "5s"))
}

func (s *EventIntSuite) TestRestartRule(c *C) {
	if err := helper.RunGonitCmd("start all", verbose,
		s.eventCfgDir); err != nil {
		c.Errorf(err.Error())
	}

	balloonPid, err := helper.ProxyReadPidFile(balloonPidFile)
	c.Check(err, IsNil)
	c.Check(helper.DoesProcessExist(c, balloonPid), Equals, true)
	pid1, _ := helper.ProxyReadPidFile(balloonPidFile)
	c.Check(true, Equals,
		helper.FindLogLine(s.stdout, "Executing 'restart'", "20s"))
	c.Check(true, Equals,
		helper.FindLogLine(s.stdout, "process \"balloonmem\" started", "10s"))
	balloonPid, err = helper.ProxyReadPidFile(balloonPidFile)
	c.Check(err, IsNil)
	c.Check(helper.DoesProcessExist(c, balloonPid), Equals, true)
	pid2, err := helper.ProxyReadPidFile(balloonPidFile)
	if err != nil {
		c.Errorf(err.Error())
	}
	c.Check(pid1, Not(Equals), pid2)
}
