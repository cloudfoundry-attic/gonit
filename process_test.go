// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"github.com/bmizerany/assert"
	"github.com/cloudfoundry/gonit/test/helper"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"
)

var dprocess, goprocess string

func init() {
	// binary used for the majority of tests
	goprocess = helper.BuildTestProgram("process")
}

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

func cleanup(daemon *Daemon) {
	os.RemoveAll(daemon.Dir)
}

func mkcmd(args []string, action string) []string {
	cmd := make([]string, len(args))
	copy(cmd, args)
	return append(cmd, action)
}

func NewTestDaemon(name string, flags []string, detached bool) *Daemon {
	// using '/tmp' rather than os.TempDir, otherwise 'sudo -E go test'
	// will fail on darwin, since only the user that started the process
	// has rx perms
	dir, err := ioutil.TempDir("/tmp", "gonit-pt-"+name)
	if err != nil {
		log.Fatal(err)
	}
	os.Chmod(dir, 0755) // rx perms for all

	// see TempDir comment; copy goprocess where any user can execute
	dprocess = filepath.Join(dir, path.Base(goprocess))
	helper.CopyFile(goprocess, dprocess, 0555)

	logfile := filepath.Join(dir, name+".log")
	pidfile := filepath.Join(dir, name+".pid")

	var start, stop, restart []string

	args := []string{
		dprocess,
		"-d", dir,
		"-n", name,
	}

	for _, arg := range flags {
		args = append(args, arg)
	}

	if detached {
		// process will detach itself
		args = append(args, "-F", "-p", pidfile)

		// configure stop + restart commands with
		// the same flags as the start command
		stop = mkcmd(args, "stop")
		restart = mkcmd(args, "restart")
	}

	start = mkcmd(args, "start")

	return &Daemon{
		Name:     name,
		Start:    strings.Join(start, " "),
		Stop:     strings.Join(stop, " "),
		Restart:  strings.Join(restart, " "),
		Dir:      dir,
		Stderr:   logfile,
		Stdout:   logfile,
		Pidfile:  pidfile,
		Detached: detached,
	}
}

// start + stop of gonit daemonized process
func TestSimple(t *testing.T) {
	daemon := NewTestDaemon("simple", nil, false)
	defer cleanup(daemon)

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

	daemon := NewTestDaemon("simple_setuid", nil, false)
	defer cleanup(daemon)

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
	daemon := NewTestDaemon("detached", nil, true)
	defer cleanup(daemon)

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

	daemon := NewTestDaemon("fail_setuid", nil, false)
	defer cleanup(daemon)

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
	daemon := NewTestDaemon("fail_exe", nil, false)
	defer cleanup(daemon)

	err := os.Chmod(dprocess, 0444)
	if err != nil {
		log.Fatal(err)
	}

	_, err = daemon.StartProcess()
	assert.Equal(t, syscall.EPERM, err)

	pause()

	assert.Equal(t, false, daemon.IsRunning())
}
