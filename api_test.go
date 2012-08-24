// Copyright (c) 2012 VMware, Inc.

package gonit_test

import (
	"bytes"
	"github.com/bmizerany/assert"
	. "github.com/cloudfoundry/gonit"
	"github.com/cloudfoundry/gonit/test/helper"
	"io/ioutil"
	"math"
	"net/rpc"
	"os"
	"strings"
	"testing"
)

var rpcName = "TestAPI"
var config = &ConfigManager{}

func init() {
	rpc.RegisterName(rpcName, NewAPI(config))
}

func isActionError(err error) bool {
	return strings.HasPrefix(err.Error(), "ActionError")
}

func TestActionsNoConfig(t *testing.T) {
	err := helper.WithRpcServer(func(c *rpc.Client) {
		reply := &ActionResult{}
		err := c.Call(rpcName+".StartAll", "", reply)
		assert.Equal(t, nil, err)
		// Total == 0 since no processes are configured
		assert.Equal(t, 0, reply.Total)
		assert.Equal(t, 0, reply.Errors)

		reply = &ActionResult{}
		err = c.Call(rpcName+".StartProcess", "foo", reply)
		assert.NotEqual(t, nil, err)
		assert.Equal(t, true, isActionError(err))

		reply = &ActionResult{}
		err = c.Call(rpcName+".StartGroup", "bar", reply)
		assert.NotEqual(t, nil, err)
		assert.Equal(t, true, isActionError(err))
	})

	assert.Equal(t, nil, err)
}

func TestAbout(t *testing.T) {
	err := helper.WithRpcServer(func(c *rpc.Client) {
		params := struct{}{}
		about := About{}
		err := c.Call(rpcName+".About", &params, &about)
		assert.Equal(t, nil, err)

		assert.Equal(t, VERSION, about.Version)

		// need not be same type, just requires same fields
		var gabout struct {
			Version string
		}

		err = c.Call(rpcName+".About", &params, &gabout)
		assert.Equal(t, nil, err)

		assert.Equal(t, VERSION, gabout.Version)
	})

	assert.Equal(t, nil, err)
}

func tmpPidFile(t *testing.T, pid int) string {
	file, err := ioutil.TempFile("", "api_test")
	if err != nil {
		t.Error(err)
	}
	if err := WritePidFile(pid, file.Name()); err != nil {
		t.Fatal(err)
	}
	return file.Name()
}

// simple exercise of CliFormatters
func testCliPrint(t *testing.T, value CliFormatter) {
	buf := new(bytes.Buffer)
	value.Print(buf)
	assert.NotEqual(t, 0, buf.Len())
}

func TestStatus(t *testing.T) {
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
			pidfile := tmpPidFile(t, process.pid)
			defer os.Remove(pidfile)

			config.AddProcess(group.name, &Process{
				Name:    process.name,
				Pidfile: pidfile,
			})

			nprocesses++
		}
	}

	err := helper.WithRpcServer(func(c *rpc.Client) {
		statusGroup := rpcName + ".StatusGroup"
		statusProcess := rpcName + ".StatusProcess"

		// should get an error if group does not exist
		err := c.Call(statusGroup, "enogroup", &ProcessGroupStatus{})
		assert.NotEqual(t, nil, err)

		// should get an error if process does not exist
		err = c.Call(statusProcess, "enoprocess", &ProcessStatus{})
		assert.NotEqual(t, nil, err)

		for _, group := range groups {
			gstat := &ProcessGroupStatus{}
			err := c.Call(statusGroup, group.name, gstat)
			assert.Equal(t, nil, err)
			assert.Equal(t, len(group.processes), len(gstat.Group))
			testCliPrint(t, gstat)

			for _, process := range group.processes {
				stat := &ProcessStatus{}
				err := c.Call(statusProcess, process.name, stat)
				assert.Equal(t, nil, err)

				running := stat.Summary.Running
				assert.Equal(t, process.running, running)
				testCliPrint(t, stat)

				if !running {
					continue
				}

				assert.Equal(t, process.pid, stat.Pid)
				assert.Equal(t, process.ppid, stat.State.Ppid)
			}
		}

		all := &ProcessGroupStatus{}
		err = c.Call(rpcName+".StatusAll", "", all)
		assert.Equal(t, nil, err)
		assert.Equal(t, nprocesses, len(all.Group))
		testCliPrint(t, all)

		summary := &Summary{}
		err = c.Call(rpcName+".Summary", "", summary)
		assert.Equal(t, nil, err)
		assert.Equal(t, nprocesses, len(summary.Processes))
		testCliPrint(t, summary)
	})

	assert.Equal(t, nil, err)
}
