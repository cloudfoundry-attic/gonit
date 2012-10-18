// Copyright (c) 2012 VMware, Inc.

package gonit_integration_test

import (
	"github.com/cloudfoundry/gonit/test/helper"
	. "launchpad.net/gocheck"
)

type EventIntSuite struct{}

var _ = Suite(&EventIntSuite{})

var balloonPidFile = "../test/integration/balloonmem.pid"

func (s *EventIntSuite) TestAlertRule(c *C) {
	simpleIntegrationDir := integrationDir + "/../config/eventmonitor_integration"
	cmd, stdout, expectedExit := helper.StartGonit(simpleIntegrationDir, verbose)
	defer helper.StopGonit(cmd, verbose, expectedExit)
	helper.RunGonitCmd("start all", verbose)
	c.Check(true, Equals, helper.FindLogLine(stdout,
		"'balloonmem' triggered 'memory_used > 1mb' for '1s'", "5s"))
}

func (s *EventIntSuite) TestRestartRule(c *C) {
	simpleIntegrationDir := integrationDir + "/../config/eventmonitor_integration"
	cmd, stdout, expectedExit := helper.StartGonit(simpleIntegrationDir, verbose)
	defer helper.StopGonit(cmd, verbose, expectedExit)
	helper.RunGonitCmd("start all", verbose)

	balloonPid, err := helper.ProxyReadPidFile(balloonPidFile)
	c.Check(nil, Equals, err)
	helper.AssertProcessExists(c, balloonPid)
	pid1, _ := helper.ProxyReadPidFile(balloonPidFile)
	c.Check(true, Equals,
		helper.FindLogLine(stdout, "Executing 'restart'", "20s"))
	c.Check(true, Equals,
		helper.FindLogLine(stdout, "process \"balloonmem\" started", "10s"))
	balloonPid, err = helper.ProxyReadPidFile(balloonPidFile)
	c.Check(nil, Equals, err)
	helper.AssertProcessExists(c, balloonPid)
	pid2, _ := helper.ProxyReadPidFile(balloonPidFile)
	c.Check(pid1, Not(Equals), pid2)
}
