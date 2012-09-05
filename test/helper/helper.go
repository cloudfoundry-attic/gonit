// Copyright (c) 2012 VMware, Inc.

package helper

import (
	"encoding/json"
	. "github.com/cloudfoundry/gonit"
	"io"
	"io/ioutil"
	"log"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

type ProcessInfo struct {
	Ppid     int
	Pid      int
	Uid      int
	Euid     int
	Gid      int
	Egid     int
	Pgrp     int
	Sid      int
	Dir      string
	Groups   []int
	Args     []string
	Env      []string
	Restarts int
	HasTty   bool
}

var TestProcess, goprocess string

func CurrentProcessInfo() *ProcessInfo {
	var hasTty bool
	cwd, _ := os.Getwd()
	grp, _ := os.Getgroups()
	// no syscall.Getsid() wrapper on Linux?
	sid, _, _ := syscall.RawSyscall(syscall.SYS_GETSID, 0, 0, 0)

	if fh, err := os.Open("/dev/tty"); err == nil {
		hasTty = true
		fh.Close()
	}

	return &ProcessInfo{
		Ppid:   os.Getppid(),
		Pid:    os.Getpid(),
		Uid:    os.Getuid(),
		Euid:   os.Geteuid(),
		Gid:    os.Getgid(),
		Egid:   os.Getegid(),
		Pgrp:   syscall.Getpgrp(),
		Sid:    int(sid),
		Dir:    cwd,
		Groups: grp,
		Args:   os.Args,
		Env:    os.Environ(),
		HasTty: hasTty,
	}
}

func toJson(data interface{}) []byte {
	json, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	return json
}

func WriteData(data interface{}, file string) {
	err := ioutil.WriteFile(file, toJson(data), 0666)
	if err != nil {
		log.Fatalf("WriteFile(%s): %v", file, err)
	}
}

func ReadData(data interface{}, file string) {
	contents, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal(contents, data)
	if err != nil {
		log.Fatal(err)
	}
}

// compile "test/$name/main.go" to go test's tmpdir
// and return path to the executable
func BuildTestProgram(name string) string {
	dir := path.Dir(os.Args[0])
	path := filepath.Join(dir, "go"+name)
	main := filepath.Join("test", name, "main.go")

	cmd := exec.Command("go", "build", "-o", path, main)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	return path
}

func TouchFile(name string, mode os.FileMode) {
	dst, err := os.Create(name)
	if err != nil {
		log.Fatal(err)
	}
	dst.Close()
	err = os.Chmod(name, mode)
	if err != nil {
		log.Fatal(err)
	}
}

func CopyFile(srcName, dstName string, mode os.FileMode) {
	src, err := os.Open(srcName)
	if err != nil {
		log.Fatal(err)
	}
	defer src.Close()

	dst, err := os.Create(dstName)
	if err != nil {
		log.Fatal(err)
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		log.Fatal(err)
	}

	err = os.Chmod(dstName, mode)
	if err != nil {
		log.Fatal(err)
	}
}

func NotRoot() bool {
	if os.Getuid() == 0 {
		return false
	}
	log.Println("SKIP: test must be run as root")
	return true
}

func WithRpcServer(f func(c *rpc.Client)) error {
	file, err := ioutil.TempFile("", "gonit-rpc")
	if err != nil {
		return err
	}
	path := file.Name()
	defer os.Remove(path)
	os.Remove(path)

	url := "unix://" + path

	server, err := NewRpcServer(url)

	if err != nil {
		return err
	}

	go server.Serve()

	client, err := jsonrpc.Dial("unix", path)

	if err != nil {
		return err
	}

	defer client.Close()

	f(client)

	server.Shutdown()

	return nil
}

func Cleanup(p *Process) {
	os.RemoveAll(p.Dir)
}

func mkcmd(args []string, action string) []string {
	cmd := make([]string, len(args))
	copy(cmd, args)
	return append(cmd, action)
}

func NewTestProcess(name string, flags []string, detached bool) *Process {
	// using '/tmp' rather than os.TempDir, otherwise 'sudo -E go test'
	// will fail on darwin, since only the user that started the process
	// has rx perms
	dir, err := ioutil.TempDir("/tmp", "gonit-pt-"+name)
	if err != nil {
		log.Fatal(err)
	}
	os.Chmod(dir, 0755) // rx perms for all

	if goprocess == "" {
		// binary used for the majority of tests
		goprocess = BuildTestProgram("process")
	}

	// see TempDir comment; copy goprocess where any user can execute
	TestProcess = filepath.Join(dir, path.Base(goprocess))
	CopyFile(goprocess, TestProcess, 0555)

	logfile := filepath.Join(dir, name+".log")
	pidfile := filepath.Join(dir, name+".pid")

	var start, stop, restart []string

	args := []string{
		TestProcess,
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

	return &Process{
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
