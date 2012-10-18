// Copyright (c) 2012 VMware, Inc.

package gonit_integration

import (
	"github.com/cloudfoundry/gonit/test/helper"
	. "launchpad.net/gocheck"
	"os"
	"os/exec"
)

type ControlIntSuite struct {
	gonitCmd   *exec.Cmd
	ctrlCfgDir string
}

var _ = Suite(&ControlIntSuite{})

var controlPidFile1 string
var controlPidFile2 string

func (s *ControlIntSuite) SetUpTest(c *C) {
	s.ctrlCfgDir = integrationDir + "/tmp/process_tmp"
	if err := os.MkdirAll(s.ctrlCfgDir, 0755); err != nil {
		c.Errorf(err.Error())
	}
	controlPidFile1 = s.ctrlCfgDir + "/control0.pid"
	controlPidFile2 = s.ctrlCfgDir + "/control1.pid"
	err := helper.CreateGonitCfg(2, "control", "./process_tmp", "./goprocess",
		false)
	if err != nil {
		c.Errorf(err.Error())
	}
	helper.CreateGonitSettings("./gonit.pid", "./", "./process_tmp")
	s.gonitCmd, _, err = helper.StartGonit(s.ctrlCfgDir)
	if err != nil {
		c.Errorf(err.Error())
	}
}

func (s *ControlIntSuite) TearDownTest(c *C) {
	if err := helper.StopGonit(s.gonitCmd, s.ctrlCfgDir); err != nil {
		c.Errorf(err.Error())
	}
	if err := os.RemoveAll(s.ctrlCfgDir); err != nil {
		c.Errorf(err.Error())
	}
}

func (s *ControlIntSuite) TestStartStop(c *C) {
	if err := helper.RunGonitCmd("start all", s.ctrlCfgDir); err != nil {
		c.Errorf(err.Error())
	}

	pid1, err := helper.ProxyReadPidFile(controlPidFile1)
	c.Check(err, IsNil)
	pid2, err := helper.ProxyReadPidFile(controlPidFile2)
	c.Check(err, IsNil)

	c.Check(helper.DoesProcessExist(pid1), Equals, true)
	c.Check(helper.DoesProcessExist(pid2), Equals, true)
	if err := helper.RunGonitCmd("stop all", s.ctrlCfgDir); err != nil {
		c.Errorf(err.Error())
	}
	c.Check(helper.DoesProcessExist(pid1), Equals, false)
	c.Check(helper.DoesProcessExist(pid2), Equals, false)
}

func (s *ControlIntSuite) TestRestart(c *C) {
	if err := helper.RunGonitCmd("start all", s.ctrlCfgDir); err != nil {
		c.Errorf(err.Error())
	}

	firstPid1, err := helper.ProxyReadPidFile(controlPidFile1)
	c.Check(err, IsNil)
	firstPid2, err := helper.ProxyReadPidFile(controlPidFile2)
	c.Check(err, IsNil)

	c.Check(helper.DoesProcessExist(firstPid1), Equals, true)
	c.Check(helper.DoesProcessExist(firstPid2), Equals, true)
	if err := helper.RunGonitCmd("restart all", s.ctrlCfgDir); err != nil {
		c.Errorf(err.Error())
	}

	secondPid1, err := helper.ProxyReadPidFile(controlPidFile1)
	c.Check(err, IsNil)
	secondPid2, err := helper.ProxyReadPidFile(controlPidFile2)
	c.Check(err, IsNil)

	c.Check(helper.DoesProcessExist(secondPid1), Equals, true)
	c.Check(helper.DoesProcessExist(secondPid2), Equals, true)
	c.Check(helper.DoesProcessExist(firstPid1), Equals, false)
	c.Check(helper.DoesProcessExist(firstPid2), Equals, false)
	c.Check(firstPid1, Not(Equals), secondPid1)
	c.Check(firstPid2, Not(Equals), secondPid2)
}
