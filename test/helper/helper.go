// Copyright (c) 2012 VMware, Inc.

package helper

import (
	"bufio"
	"encoding/json"
	"fmt"
	. "github.com/cloudfoundry/gonit"
	"github.com/cloudfoundry/gosteno"
	"io"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"log"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"
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

// Read pid from a file
func ProxyReadPidFile(path string) (int, error) {
	return ReadPidFile(path)
}

func BuildGonitMain(path string) {
	if err := os.Chdir(path); err != nil {
		fmt.Printf("Error changing directory: %v", err.Error())
		os.Exit(2)
	}
	output, err := exec.Command("go", "build").Output()
	if err != nil {
		fmt.Printf("Error building gonit: \n%v\n", string(output))
		os.Exit(2)
	}
}

func watchGonit(gonitCmd *exec.Cmd, stderr *io.ReadCloser) {
	// todo use stderr here
	processState, err := gonitCmd.Process.Wait()
	if err != nil || processState.String() != "signal 9" {
		fmt.Printf("Error starting gonit: %+v\n", err)
		os.Exit(2)
	}
}

func StartGonit(configDir string, silent bool) (*exec.Cmd, *io.ReadCloser) {
	cmd := exec.Command("./gonit", "-d", "10", "-c", configDir)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Println(err)
	}
	if err := cmd.Start(); err != nil {
		fmt.Printf("Error starting gonit: %v\n", err.Error())
		os.Exit(2)
	}
	if !silent {
		go io.Copy(os.Stdout, stdout)
		go io.Copy(os.Stderr, stderr)
	}
	go watchGonit(cmd, &stderr)
	waitUntilRunning()
	return cmd, &stdout
}

func StopGonit(gonitCmd *exec.Cmd, silent bool) {
	cmd := exec.Command("./gonit", "stop", "all")
	StartAndPipeOutput(cmd, silent)
	cmd.Wait()
	fmt.Printf("killing gonit pid %v\n", gonitCmd.Process.Pid)
	gonitCmd.Process.Kill()
	fmt.Printf("killed.\n")
}

func StartAndPipeOutput(cmd *exec.Cmd, silent bool) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Println(err)
	}
	err = cmd.Start()
	if err != nil {
		fmt.Println(err)
	}
	if !silent {
		go io.Copy(os.Stdout, stdout)
		go io.Copy(os.Stderr, stderr)
	}
}

func waitUntilRunning() {
	_, err := exec.Command("./gonit", "IsRunning").Output()
	if err != nil {
		fmt.Printf("Waiting for gonit.\n")
		waitUntilRunning()
	} else {
		fmt.Printf("Gonit is ready.\n")
	}
}

func AssertProcessExists(c *C, pidfile string) {
	pid, err := ProxyReadPidFile(pidfile)
	c.Check(nil, Equals, err)
	_, err = syscall.Getpgid(pid)
	c.Check(nil, Equals, err)
}

func AssertProcessDoesntExist(c *C, pidfile string) {
	pid, err := ProxyReadPidFile(pidfile)
	c.Check(nil, Equals, err)
	_, err = syscall.Getpgid(pid)
	c.Check(nil, Not(Equals), err)
}

func setTimedOut(duration time.Duration, timedout *bool) {
	for {
		select {
		case <-time.After(duration):
			*timedout = true
			break
		}
	}
}

func FindLogLine(stdout *io.ReadCloser, logline string, timeout string) bool {
	duration, err := time.ParseDuration(timeout)
	if err != nil {
		fmt.Printf("Invalid duration '%v'", timeout)
		return false
	}
	reader := bufio.NewReader(*stdout)
	var line []byte
	timedout := false
	go setTimedOut(duration, &timedout)
	prettifier := steno.NewJsonPrettifier(steno.EXCLUDE_NONE)

	for ; err == nil; line, _, err = reader.ReadLine() {
		if timedout {
			return false
		}
		if err != nil {
			fmt.Printf("error! %v\n", err.Error())
		}
		record, err := prettifier.DecodeJsonLogEntry(string(line))
		if err != nil {
			// If we have an error, it's likely because the configmanager logs some
			// stuff before the steno log is told to output JSON, so we get some
			// non-JSON messages that can't be parsed.
			continue
		}
		fmt.Printf("Message: %v\nLogline: %v\nContains: %v\n", record.Message,
			logline, strings.Contains(record.Message, logline))
		if strings.Contains(record.Message, logline) {
			return true
		}
	}
	return false
}
