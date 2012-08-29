// Copyright (c) 2012 VMware, Inc.

package gonit_test

import (
	"github.com/bmizerany/assert"
	. "github.com/cloudfoundry/gonit"
	"github.com/cloudfoundry/gonit/test/helper"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"
)

// absolute path to the json encoded helper.ProcessInfo
// written by goprocess, read by the following Tests.
func processJsonFile(p *Process) string {
	return filepath.Join(p.Dir, p.Name+".json")
}

func processInfo(p *Process) *helper.ProcessInfo {
	file := processJsonFile(p)
	info := &helper.ProcessInfo{}
	helper.ReadData(info, file)
	return info
}

// these assertions apply to any daemon process
func assertProcessInfo(t *testing.T, process *Process, info *helper.ProcessInfo) {
	selfInfo := helper.CurrentProcessInfo()

	assert.Equal(t, 1, info.Ppid) // parent pid is init (1)

	assert.NotEqual(t, selfInfo.Pgrp, info.Pgrp) // process group will change

	assert.NotEqual(t, selfInfo.Sid, info.Sid) // session id will change

	sort.Strings(selfInfo.Env)
	sort.Strings(info.Env)
	assert.NotEqual(t, selfInfo.Env, info.Env) // sanitized env will differ

	// check expected process working directory
	// (and follow symlinks, e.g. darwin)
	ddir, _ := filepath.EvalSymlinks(process.Dir)
	idir, _ := filepath.EvalSymlinks(info.Dir)
	assert.Equal(t, ddir, idir)

	// spawned process argv[] should be the same as the process.Start command
	assert.Equal(t, process.Start, strings.Join(info.Args, " "))

	// check when configured to run as different user and/or group
	if process.Uid == "" {
		assert.Equal(t, selfInfo.Uid, info.Uid)
		assert.Equal(t, selfInfo.Euid, info.Euid)
	} else {
		assert.NotEqual(t, selfInfo.Uid, info.Uid)
		assert.NotEqual(t, selfInfo.Euid, info.Euid)
	}

	if process.Gid == "" {
		if process.Uid == "" {
			assert.Equal(t, selfInfo.Gid, info.Gid)
			assert.Equal(t, selfInfo.Egid, info.Egid)
		} else {
			assert.NotEqual(t, selfInfo.Gid, info.Gid)
			assert.NotEqual(t, selfInfo.Egid, info.Egid)
		}
	} else {
		assert.NotEqual(t, selfInfo.Gid, info.Gid)
		assert.NotEqual(t, selfInfo.Egid, info.Egid)
	}
}

// lame, but need wait a few ticks for processes to start,
// files to write, etc.
func pause() {
	time.Sleep(100 * time.Millisecond)
}

// start + stop of gonit daemonized process
func TestSimple(t *testing.T) {
	process := helper.NewTestProcess("simple", nil, false)
	defer helper.Cleanup(process)

	pid, err := process.StartProcess()
	if err != nil {
		log.Fatal(err)
	}

	pause()

	assert.Equal(t, true, process.IsRunning())

	info := processInfo(process)

	if process.Detached {
		assert.Equal(t, true, grandArgs(info.Args))
	} else {
		assert.Equal(t, pid, info.Pid)
	}
	assertProcessInfo(t, process, info)

	err = process.StopProcess()
	assert.Equal(t, nil, err)

	pause()

	assert.Equal(t, false, process.IsRunning())
}

// start + stop of gonit daemonized process w/ setuid
func TestSimpleSetuid(t *testing.T) {
	if helper.NotRoot() {
		return
	}

	process := helper.NewTestProcess("simple_setuid", nil, false)
	defer helper.Cleanup(process)

	helper.TouchFile(processJsonFile(process), 0666)

	process.Uid = "nobody"
	process.Gid = "nogroup"

	pid, err := process.StartProcess()
	if err != nil {
		t.Fatal(err)
	}

	pause()

	assert.Equal(t, true, process.IsRunning())

	info := processInfo(process)
	assertProcessInfo(t, process, info)
	assert.Equal(t, pid, info.Pid)

	err = process.StopProcess()
	assert.Equal(t, nil, err)

	pause()

	assert.Equal(t, false, process.IsRunning())
}

// check that -F flag has been rewritten to -G
// and reset args to -F so assertProcessInfo()
// can check the rest of argv[]
func grandArgs(args []string) bool {
	for i, arg := range args {
		if arg == "-G" {
			args[i] = "-F"
			return true
		}
	}
	return false
}

// start / restart / stop self-daemonized process
func TestDetached(t *testing.T) {
	// start process
	process := helper.NewTestProcess("detached", nil, true)
	defer helper.Cleanup(process)

	pid, err := process.StartProcess()
	if err != nil {
		log.Fatal(err)
	}

	pause()

	assert.Equal(t, true, process.IsRunning())

	info := processInfo(process)

	assert.Equal(t, true, grandArgs(info.Args))
	assertProcessInfo(t, process, info)
	assert.NotEqual(t, pid, info.Pid)
	pid, err = process.Pid()
	if err != nil {
		log.Fatal(err)
	}
	assert.Equal(t, pid, info.Pid)

	// restart via SIGHUP
	prevPid := info.Pid

	assert.Equal(t, 0, info.Restarts)

	for i := 1; i < 3; i++ {
		err = process.RestartProcess()

		pause()
		assert.Equal(t, true, process.IsRunning())

		pid, err = process.Pid()
		if err != nil {
			log.Fatal(err)
		}

		assert.Equal(t, prevPid, pid)
		info = processInfo(process)
		assert.Equal(t, true, grandArgs(info.Args))
		assertProcessInfo(t, process, info)

		// SIGHUP increments restarts counter
		assert.Equal(t, i, info.Restarts)
	}

	// restart via full stop+start
	prevPid = info.Pid

	process.Restart = ""

	err = process.RestartProcess()

	pause()
	assert.Equal(t, true, process.IsRunning())

	pid, err = process.Pid()
	if err != nil {
		log.Fatal(err)
	}

	assert.NotEqual(t, prevPid, pid)
	info = processInfo(process)
	assert.Equal(t, true, grandArgs(info.Args))
	assertProcessInfo(t, process, info)

	err = process.StopProcess()
	assert.Equal(t, nil, err)

	pause()

	assert.Equal(t, false, process.IsRunning())
}

// test invalid uid
func TestFailSetuid(t *testing.T) {
	if helper.NotRoot() {
		return
	}

	process := helper.NewTestProcess("fail_setuid", nil, false)
	defer helper.Cleanup(process)

	process.Uid = "aint_nobody"

	_, err := process.StartProcess()
	if err == nil {
		t.Fatalf("user.LookupId(%q) should have failed", process.Uid)
	}

	pause()

	assert.Equal(t, false, process.IsRunning())
}

// test invalid executable
func TestFailExe(t *testing.T) {
	process := helper.NewTestProcess("fail_exe", nil, false)
	defer helper.Cleanup(process)

	err := os.Chmod(helper.TestProcess, 0444)
	if err != nil {
		log.Fatal(err)
	}

	_, err = process.StartProcess()
	if process.Detached {
		assert.NotEqual(t, nil, err)
	} else {
		assert.Equal(t, syscall.EPERM, err)
	}
	pause()

	assert.Equal(t, false, process.IsRunning())
}
