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
func processJsonFile(d *Daemon) string {
	return filepath.Join(d.Dir, d.Name+".json")
}

func processInfo(d *Daemon) *helper.ProcessInfo {
	file := processJsonFile(d)
	info := &helper.ProcessInfo{}
	helper.ReadData(info, file)
	return info
}

// these assertions apply to any daemon process
func assertProcessInfo(t *testing.T, daemon *Daemon, info *helper.ProcessInfo) {
	selfInfo := helper.CurrentProcessInfo()

	assert.Equal(t, 1, info.Ppid) // parent pid is init (1)

	assert.NotEqual(t, selfInfo.Pgrp, info.Pgrp) // process group will change

	assert.NotEqual(t, selfInfo.Sid, info.Sid) // session id will change

	sort.Strings(selfInfo.Env)
	sort.Strings(info.Env)
	assert.NotEqual(t, selfInfo.Env, info.Env) // sanitized env will differ

	// check expected process working directory
	// (and follow symlinks, e.g. darwin)
	ddir, _ := filepath.EvalSymlinks(daemon.Dir)
	idir, _ := filepath.EvalSymlinks(info.Dir)
	assert.Equal(t, ddir, idir)

	// spawned process argv[] should be the same as the daemon.Start command
	assert.Equal(t, daemon.Start, strings.Join(info.Args, " "))

	// check when configured to run as different user and/or group
	if daemon.User == "" {
		assert.Equal(t, selfInfo.Uid, info.Uid)
		assert.Equal(t, selfInfo.Euid, info.Euid)
	} else {
		assert.NotEqual(t, selfInfo.Uid, info.Uid)
		assert.NotEqual(t, selfInfo.Euid, info.Euid)
	}

	if daemon.Group == "" {
		if daemon.User == "" {
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
	daemon := helper.NewTestDaemon("simple", nil, false)
	defer helper.Cleanup(daemon)

	pid, err := daemon.StartProcess()
	if err != nil {
		log.Fatal(err)
	}

	pause()

	assert.Equal(t, true, daemon.IsRunning())

	info := processInfo(daemon)
	assertProcessInfo(t, daemon, info)
	assert.Equal(t, pid, info.Pid)

	err = daemon.StopProcess()
	assert.Equal(t, nil, err)

	pause()

	assert.Equal(t, false, daemon.IsRunning())
}

// start + stop of gonit daemonized process w/ setuid
func TestSimpleSetuid(t *testing.T) {
	if helper.NotRoot() {
		return
	}

	daemon := helper.NewTestDaemon("simple_setuid", nil, false)
	defer helper.Cleanup(daemon)

	helper.TouchFile(processJsonFile(daemon), 0666)

	daemon.User = "nobody"
	daemon.Group = "nogroup"

	pid, err := daemon.StartProcess()
	if err != nil {
		t.Fatal(err)
	}

	pause()

	assert.Equal(t, true, daemon.IsRunning())

	info := processInfo(daemon)
	assertProcessInfo(t, daemon, info)
	assert.Equal(t, pid, info.Pid)

	err = daemon.StopProcess()
	assert.Equal(t, nil, err)

	pause()

	assert.Equal(t, false, daemon.IsRunning())
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
	daemon := helper.NewTestDaemon("detached", nil, true)
	defer helper.Cleanup(daemon)

	pid, err := daemon.StartProcess()
	if err != nil {
		log.Fatal(err)
	}

	pause()

	assert.Equal(t, true, daemon.IsRunning())

	info := processInfo(daemon)

	assert.Equal(t, true, grandArgs(info.Args))
	assertProcessInfo(t, daemon, info)
	assert.NotEqual(t, pid, info.Pid)
	pid, err = daemon.Pid()
	if err != nil {
		log.Fatal(err)
	}
	assert.Equal(t, pid, info.Pid)

	// restart via SIGHUP
	prevPid := info.Pid

	assert.Equal(t, 0, info.Restarts)

	for i := 1; i < 3; i++ {
		err = daemon.RestartProcess()

		pause()
		assert.Equal(t, true, daemon.IsRunning())

		pid, err = daemon.Pid()
		if err != nil {
			log.Fatal(err)
		}

		assert.Equal(t, prevPid, pid)
		info = processInfo(daemon)
		assert.Equal(t, true, grandArgs(info.Args))
		assertProcessInfo(t, daemon, info)

		// SIGHUP increments restarts counter
		assert.Equal(t, i, info.Restarts)
	}

	// restart via full stop+start
	prevPid = info.Pid

	daemon.Restart = ""

	err = daemon.RestartProcess()

	pause()
	assert.Equal(t, true, daemon.IsRunning())

	pid, err = daemon.Pid()
	if err != nil {
		log.Fatal(err)
	}

	assert.NotEqual(t, prevPid, pid)
	info = processInfo(daemon)
	assert.Equal(t, true, grandArgs(info.Args))
	assertProcessInfo(t, daemon, info)

	err = daemon.StopProcess()
	assert.Equal(t, nil, err)

	pause()

	assert.Equal(t, false, daemon.IsRunning())
}

// test invalid uid
func TestFailSetuid(t *testing.T) {
	if helper.NotRoot() {
		return
	}

	daemon := helper.NewTestDaemon("fail_setuid", nil, false)
	defer helper.Cleanup(daemon)

	daemon.User = "aint_nobody"

	_, err := daemon.StartProcess()
	if err == nil {
		t.Fatalf("user.LookupId(%q) should have failed", daemon.User)
	}

	pause()

	assert.Equal(t, false, daemon.IsRunning())
}

// test invalid executable
func TestFailExe(t *testing.T) {
	daemon := helper.NewTestDaemon("fail_exe", nil, false)
	defer helper.Cleanup(daemon)

	err := os.Chmod(helper.TestProcess, 0444)
	if err != nil {
		log.Fatal(err)
	}

	_, err = daemon.StartProcess()
	assert.Equal(t, syscall.EPERM, err)

	pause()

	assert.Equal(t, false, daemon.IsRunning())
}
