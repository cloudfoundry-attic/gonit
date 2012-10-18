// Copyright (c) 2012 VMware, Inc.

package gonit_test

import (
	"bytes"
	. "github.com/cloudfoundry/gonit"
	"github.com/cloudfoundry/gonit/test/helper"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"math"
	"net/rpc"
	"os"
	"strings"
	"testing"
)

type ApiSuite struct{}

var _ = Suite(&ApiSuite{})

// Hook up gocheck into the gotest runner.
func Test(t *testing.T) {
	TestingT(t)
}

var rpcName = "TestAPI"
var config = &ConfigManager{}

func init() {
	rpc.RegisterName(rpcName, NewAPI(config))
}

func isActionError(err error) bool {
	return strings.HasPrefix(err.Error(), "ActionError")
}

func (s *ApiSuite) TestActionsNoConfig(c *C) {
	err := helper.WithRpcServer(func(client *rpc.Client) {
		reply := &ActionResult{}
		err := client.Call(rpcName+".StartAll", "", reply)
		c.Check(err, IsNil)
		// Total == 0 since no processes are configured
		c.Check(0, Equals, reply.Total)
		c.Check(0, Equals, reply.Errors)

		reply = &ActionResult{}
		err = client.Call(rpcName+".StartProcess", "foo", reply)
		c.Check(err, NotNil)
		c.Check(true, Equals, isActionError(err))

		reply = &ActionResult{}
		err = client.Call(rpcName+".StartGroup", "bar", reply)
		c.Check(err, NotNil)
		c.Check(true, Equals, isActionError(err))
	})

	c.Check(err, IsNil)
}

func (s *ApiSuite) TestAbout(c *C) {
	err := helper.WithRpcServer(func(client *rpc.Client) {
		params := struct{}{}
		about := About{}
		err := client.Call(rpcName+".About", &params, &about)
		c.Check(err, IsNil)

		c.Check(VERSION, Equals, about.Version)

		// need not be same type, just requires same fields
		var gabout struct {
			Version string
		}

		err = client.Call(rpcName+".About", &params, &gabout)
		c.Check(err, IsNil)

		c.Check(VERSION, Equals, gabout.Version)
	})

	c.Check(err, IsNil)
}

func tmpPidFile(c *C, pid int) string {
	file, err := ioutil.TempFile("", "api_test")
	if err != nil {
		c.Error(err)
	}
	if err := WritePidFile(pid, file.Name()); err != nil {
		c.Fatal(err)
	}
	return file.Name()
}

// simple exercise of CliFormatters
func testCliPrint(c *C, value CliFormatter) {
	buf := new(bytes.Buffer)
	value.Print(buf)
	c.Check(0, Not(Equals), buf.Len())
}

func (s *ApiSuite) TestStatus(c *C) {
	nprocesses := 0

	type testProcess struct {
		name    string
		pid     int
		ppid    int
		running bool
	}

	// use pid/ppid of the go test process to test
	// Running, sigar.ProcState, etc.
	pid := os.Getpid()
	ppid := os.Getppid()

	groups := []struct {
		name      string
		processes []testProcess
	}{
		{
			name: "a_group",
			processes: []testProcess{
				{"a_process", pid, ppid, true},
				{"x_process", math.MaxInt32, -1, false},
			},
		},
		{
			name: "b_group",
			processes: []testProcess{
				{"b_process", pid, ppid, true},
			},
		},
	}

	for _, group := range groups {
		for _, process := range group.processes {
			// write pidfile for use by process.IsRunning()
			pidfile := tmpPidFile(c, process.pid)
			defer os.Remove(pidfile)

			config.AddProcess(group.name, &Process{
				Name:    process.name,
				Pidfile: pidfile,
			})

			nprocesses++
		}
	}

	err := helper.WithRpcServer(func(client *rpc.Client) {
		statusGroup := rpcName + ".StatusGroup"
		statusProcess := rpcName + ".StatusProcess"

		// should get an error if group does not exist
		err := client.Call(statusGroup, "enogroup", &ProcessGroupStatus{})
		c.Check(err, NotNil)

		// should get an error if process does not exist
		err = client.Call(statusProcess, "enoprocess", &ProcessStatus{})
		c.Check(err, NotNil)

		for _, group := range groups {
			gstat := &ProcessGroupStatus{}
			err := client.Call(statusGroup, group.name, gstat)
			c.Check(err, IsNil)
			c.Check(len(group.processes), Equals, len(gstat.Group))
			testCliPrint(c, gstat)

			for _, process := range group.processes {
				stat := &ProcessStatus{}
				err := client.Call(statusProcess, process.name, stat)
				c.Check(err, IsNil)

				running := stat.Summary.Running
				c.Check(process.running, Equals, running)
				testCliPrint(c, stat)

				if !running {
					continue
				}

				c.Check(process.pid, Equals, stat.Pid)
				c.Check(process.ppid, Equals, stat.State.Ppid)
			}
		}

		all := &ProcessGroupStatus{}
		err = client.Call(rpcName+".StatusAll", "", all)
		c.Check(err, IsNil)
		c.Check(nprocesses, Equals, len(all.Group))
		testCliPrint(c, all)

		summary := &Summary{}
		err = client.Call(rpcName+".Summary", "", summary)
		c.Check(err, IsNil)
		c.Check(nprocesses, Equals, len(summary.Processes))
		testCliPrint(c, summary)
	})

	c.Check(err, IsNil)
}
