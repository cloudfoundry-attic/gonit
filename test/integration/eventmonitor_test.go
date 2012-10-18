// Copyright (c) 2012 VMware, Inc.

package gonit_integration_test

import (
	"flag"
	"fmt"
	"github.com/cloudfoundry/gonit/test/helper"
	"io"
	. "launchpad.net/gocheck"
	"os"
	"os/exec"
	"testing"
)

type EventIntSuite struct{}

var _ = Suite(&EventIntSuite{})

func Test(t *testing.T) {
	// propagate 'go test -v' to '-gocheck.v'
	if flag.Lookup("test.v").Value.String() == "true" {
		flag.Lookup("gocheck.v").Value.Set("true")
	}
	flag.Parse()
	if intDir, err := os.Getwd(); err != nil {
		fmt.Printf("Error getting pwd: %v", err.Error())
		os.Exit(2)
	} else {
		integrationDir = intDir
	}
	helper.BuildGonitMain(integrationDir + "/../../gonit")
	TestingT(t)
}

var (
	gonitCmd       *exec.Cmd
	integrationDir string
	stdout         io.ReadCloser
	stderr         io.ReadCloser
	pidFile        = "/Users/lisbakke/Documents/work/mygo/src/github.com/cloudfoundry/gonit/test/demo/balloonmem.pid"
	silent         = flag.Bool("s", false, "Silence log output.")
)

func (s *EventIntSuite) TestAlertRule(c *C) {
	simpleIntegrationDir := integrationDir + "/../config/simple_integration"
	cmd, stdout := helper.StartGonit(simpleIntegrationDir, *silent)
	defer helper.StopGonit(cmd, *silent)
	cmd = exec.Command("./gonit", "start", "all")
	helper.StartAndPipeOutput(cmd, *silent)
	cmd.Wait()
	c.Check(true, Equals,
		helper.FindLogLine(stdout, "'balloonmem' triggered 'memory_used > 1mb' for '1s'", "5s"))
}

func (s *EventIntSuite) TestRestartRule(c *C) {
	simpleIntegrationDir := integrationDir + "/../config/simple_integration"
	cmd, stdout := helper.StartGonit(simpleIntegrationDir, *silent)
	defer helper.StopGonit(cmd, *silent)
	cmd = exec.Command("./gonit", "start", "all")
	helper.StartAndPipeOutput(cmd, *silent)
	cmd.Wait()
	helper.AssertProcessExists(c, pidFile)
	pid1, _ := helper.ProxyReadPidFile(pidFile)
	c.Check(true, Equals,
		helper.FindLogLine(stdout, "Executing 'restart'", "20s"))
	c.Check(true, Equals,
		helper.FindLogLine(stdout, "process \"balloonmem\" started", "5s"))
	helper.AssertProcessExists(c, pidFile)
	pid2, _ := helper.ProxyReadPidFile(pidFile)
	c.Check(pid1, Not(Equals), pid2)
}

func (s *EventIntSuite) TestStartStop(c *C) {
	simpleIntegrationDir := integrationDir + "/../config/simple_integration"
	// TODO: make this part of a setup and teardown?
	cmd, _ := helper.StartGonit(simpleIntegrationDir, *silent)
	defer helper.StopGonit(cmd, *silent)
	// TODO: make these 3 lines one command.
	cmd = exec.Command("./gonit", "start", "all")
	helper.StartAndPipeOutput(cmd, *silent)
	cmd.Wait()
	helper.AssertProcessExists(c, pidFile)
	cmd = exec.Command("./gonit", "stop", "all")
	helper.StartAndPipeOutput(cmd, *silent)
	cmd.Wait()
	helper.AssertProcessDoesntExist(c, pidFile)
}
