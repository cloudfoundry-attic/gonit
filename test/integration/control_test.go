// Copyright (c) 2012 VMware, Inc.

package gonit_integration

import (
	"github.com/cloudfoundry/gonit/test/helper"
	. "launchpad.net/gocheck"
	"log"
	"os/exec"
)

type ControlIntSuite struct {
	gonitCmd     *exec.Cmd
	expectedExit *bool
	ctrlCfgDir   string
}

var _ = Suite(&ControlIntSuite{})

var controlPidFile1 = "../test/integration/tmp/process_tmp/control1.pid"
var controlPidFile2 = "../test/integration/tmp/process_tmp/control2.pid"

func (s *ControlIntSuite) SetUpTest(c *C) {
	s.ctrlCfgDir = integrationDir + "/control_integration_cfg"
	var err error
	s.gonitCmd, _, s.expectedExit, err = helper.StartGonit(s.ctrlCfgDir, verbose)
	if err != nil {
		log.Printf(err.Error())
		c.Fail()
	}
}

func (s *ControlIntSuite) TearDownTest(c *C) {
	if err := helper.StopGonit(s.gonitCmd, verbose, s.expectedExit,
		s.ctrlCfgDir); err != nil {
		c.Errorf(err.Error())
	}
	helper.RemoveFilesWithExtension(integrationDir+"/tmp/process_tmp", ".pid")
	helper.RemoveFilesWithExtension(integrationDir+"/tmp/process_tmp", ".json")
}

func (s *ControlIntSuite) TestStartStop(c *C) {
	if err := helper.RunGonitCmd("start all", verbose, s.ctrlCfgDir); err != nil {
		c.Errorf(err.Error())
	}

	pid1, err := helper.ProxyReadPidFile(controlPidFile1)
	c.Check(err, IsNil)
	pid2, err := helper.ProxyReadPidFile(controlPidFile2)
	c.Check(err, IsNil)

	c.Check(helper.DoesProcessExist(c, pid1), Equals, true)
	c.Check(helper.DoesProcessExist(c, pid2), Equals, true)
	if err := helper.RunGonitCmd("stop all", verbose, s.ctrlCfgDir); err != nil {
		c.Errorf(err.Error())
	}
	c.Check(helper.DoesProcessExist(c, pid1), Equals, false)
	c.Check(helper.DoesProcessExist(c, pid2), Equals, false)
}

func (s *ControlIntSuite) TestRestart(c *C) {
	if err := helper.RunGonitCmd("start all", verbose, s.ctrlCfgDir); err != nil {
		c.Errorf(err.Error())
	}

	firstPid1, err := helper.ProxyReadPidFile(controlPidFile1)
	c.Check(err, IsNil)
	firstPid2, err := helper.ProxyReadPidFile(controlPidFile2)
	c.Check(err, IsNil)

	c.Check(helper.DoesProcessExist(c, firstPid1), Equals, true)
	c.Check(helper.DoesProcessExist(c, firstPid2), Equals, true)
	if err := helper.RunGonitCmd("restart all", verbose,
		s.ctrlCfgDir); err != nil {
		c.Errorf(err.Error())
	}

	secondPid1, err := helper.ProxyReadPidFile(controlPidFile1)
	c.Check(err, IsNil)
	secondPid2, err := helper.ProxyReadPidFile(controlPidFile2)
	c.Check(err, IsNil)

	c.Check(helper.DoesProcessExist(c, secondPid1), Equals, true)
	c.Check(helper.DoesProcessExist(c, secondPid2), Equals, true)
	c.Check(helper.DoesProcessExist(c, firstPid1), Equals, false)
	c.Check(helper.DoesProcessExist(c, firstPid2), Equals, false)
	c.Check(firstPid1, Not(Equals), secondPid1)
	c.Check(firstPid2, Not(Equals), secondPid2)
}
