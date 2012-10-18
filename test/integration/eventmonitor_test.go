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
}

var _ = Suite(&EventIntSuite{})

var balloonPidFile = "../test/integration/balloonmem.pid"

func (s *EventIntSuite) SetUpTest(c *C) {
	intCfgDir := integrationDir + "/../config/eventmonitor_integration"
	s.gonitCmd, s.stdout, s.expectedExit = helper.StartGonit(intCfgDir, verbose)
}

func (s *EventIntSuite) TearDownTest(c *C) {
	helper.StopGonit(s.gonitCmd, verbose, s.expectedExit)
	helper.RemoveFilesWithExtension(integrationDir, ".pid")
	helper.RemoveFilesWithExtension(integrationDir + "/..", ".json")
}

func (s *EventIntSuite) TestAlertRule(c *C) {
	helper.RunGonitCmd("start all", verbose)
	c.Check(true, Equals, helper.FindLogLine(s.stdout,
		"'balloonmem' triggered 'memory_used > 1mb' for '1s'", "5s"))
}

func (s *EventIntSuite) TestRestartRule(c *C) {
	helper.RunGonitCmd("start all", verbose)

	balloonPid, err := helper.ProxyReadPidFile(balloonPidFile)
	c.Check(err, IsNil)
	helper.AssertProcessExists(c, balloonPid)
	pid1, _ := helper.ProxyReadPidFile(balloonPidFile)
	c.Check(true, Equals,
		helper.FindLogLine(s.stdout, "Executing 'restart'", "20s"))
	c.Check(true, Equals,
		helper.FindLogLine(s.stdout, "process \"balloonmem\" started", "10s"))
	balloonPid, err = helper.ProxyReadPidFile(balloonPidFile)
	c.Check(err, IsNil)
	helper.AssertProcessExists(c, balloonPid)
	pid2, _ := helper.ProxyReadPidFile(balloonPidFile)
	c.Check(pid1, Not(Equals), pid2)
}
