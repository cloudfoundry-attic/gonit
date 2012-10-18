// Copyright (c) 2012 VMware, Inc.

package gonit_integration_test

import (
	"github.com/cloudfoundry/gonit/test/helper"
	. "launchpad.net/gocheck"
)

type ControlIntSuite struct{}

var _ = Suite(&ControlIntSuite{})

var controlPidFile1 = "../test/integration/control1.pid"
var controlPidFile2 = "../test/integration/control2.pid"

func (s *ControlIntSuite) TestStartStop(c *C) {
	simpleIntegrationDir := integrationDir + "/../config/control_integration"
	cmd, _, expectedExit := helper.StartGonit(simpleIntegrationDir, verbose)
	defer helper.StopGonit(cmd, verbose, expectedExit)
	helper.RunGonitCmd("start all", verbose)

	pid1, err := helper.ProxyReadPidFile(controlPidFile1)
	c.Check(nil, Equals, err)
	pid2, err := helper.ProxyReadPidFile(controlPidFile2)
	c.Check(nil, Equals, err)

	helper.AssertProcessExists(c, pid1)
	helper.AssertProcessExists(c, pid2)
	helper.RunGonitCmd("stop all", verbose)
	helper.AssertProcessDoesntExist(c, pid1)
	helper.AssertProcessDoesntExist(c, pid2)
}

func (s *ControlIntSuite) TestRestart(c *C) {
	simpleIntegrationDir := integrationDir + "/../config/control_integration"
	cmd, _, expectedExit := helper.StartGonit(simpleIntegrationDir, verbose)
	defer helper.StopGonit(cmd, verbose, expectedExit)
	helper.RunGonitCmd("start all", verbose)

	firstPid1, err := helper.ProxyReadPidFile(controlPidFile1)
	c.Check(nil, Equals, err)
	firstPid2, err := helper.ProxyReadPidFile(controlPidFile2)
	c.Check(nil, Equals, err)

	helper.AssertProcessExists(c, firstPid1)
	helper.AssertProcessExists(c, firstPid2)
	helper.RunGonitCmd("restart all", verbose)

	secondPid1, err := helper.ProxyReadPidFile(controlPidFile1)
	c.Check(nil, Equals, err)
	secondPid2, err := helper.ProxyReadPidFile(controlPidFile2)
	c.Check(nil, Equals, err)

	helper.AssertProcessExists(c, secondPid1)
	helper.AssertProcessExists(c, secondPid2)
	helper.AssertProcessDoesntExist(c, firstPid1)
	helper.AssertProcessDoesntExist(c, firstPid2)
	c.Check(firstPid1, Not(Equals), secondPid1)
	c.Check(firstPid2, Not(Equals), secondPid2)
}
