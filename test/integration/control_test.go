// Copyright (c) 2012 VMware, Inc.

package gonit_integration

import (
	"github.com/cloudfoundry/gonit/test/helper"
	. "launchpad.net/gocheck"
	"os/exec"
)

type ControlIntSuite struct {
	gonitCmd     *exec.Cmd
	expectedExit *bool
}

var _ = Suite(&ControlIntSuite{})

var controlPidFile1 = "../test/integration/control1.pid"
var controlPidFile2 = "../test/integration/control2.pid"

func (s *ControlIntSuite) SetUpTest(c *C) {
	intCfgDir := integrationDir + "/../config/control_integration"
	s.gonitCmd, _, s.expectedExit = helper.StartGonit(intCfgDir, verbose)
}

func (s *ControlIntSuite) TearDownTest(c *C) {
	helper.StopGonit(s.gonitCmd, verbose, s.expectedExit)
	helper.RemoveFilesWithExtension(integrationDir, ".pid")
	helper.RemoveFilesWithExtension(integrationDir+"/..", ".json")
}

func (s *ControlIntSuite) TestStartStop(c *C) {
	helper.RunGonitCmd("start all", verbose)

	pid1, err := helper.ProxyReadPidFile(controlPidFile1)
	c.Check(err, IsNil)
	pid2, err := helper.ProxyReadPidFile(controlPidFile2)
	c.Check(err, IsNil)

	helper.AssertProcessExists(c, pid1)
	helper.AssertProcessExists(c, pid2)
	helper.RunGonitCmd("stop all", verbose)
	helper.AssertProcessDoesntExist(c, pid1)
	helper.AssertProcessDoesntExist(c, pid2)
}

func (s *ControlIntSuite) TestRestart(c *C) {
	helper.RunGonitCmd("start all", verbose)

	firstPid1, err := helper.ProxyReadPidFile(controlPidFile1)
	c.Check(err, IsNil)
	firstPid2, err := helper.ProxyReadPidFile(controlPidFile2)
	c.Check(err, IsNil)

	helper.AssertProcessExists(c, firstPid1)
	helper.AssertProcessExists(c, firstPid2)
	helper.RunGonitCmd("restart all", verbose)

	secondPid1, err := helper.ProxyReadPidFile(controlPidFile1)
	c.Check(err, IsNil)
	secondPid2, err := helper.ProxyReadPidFile(controlPidFile2)
	c.Check(err, IsNil)

	helper.AssertProcessExists(c, secondPid1)
	helper.AssertProcessExists(c, secondPid2)
	helper.AssertProcessDoesntExist(c, firstPid1)
	helper.AssertProcessDoesntExist(c, firstPid2)
	c.Check(firstPid1, Not(Equals), secondPid1)
	c.Check(firstPid2, Not(Equals), secondPid2)
}
