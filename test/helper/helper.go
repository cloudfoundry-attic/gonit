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
		if cwd, err := os.Getwd(); err != nil {
			log.Fatal(err)
		} else {
			goprocess = path.Dir(os.Args[0])+"/goprocess"
			err := BuildBin(cwd+"/test/process", goprocess)
			if err != nil {
				log.Fatal(err)
			}
		}
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

// Read pid from a file.
func ProxyReadPidFile(path string) (int, error) {
	return ReadPidFile(path)
}

// Given the path to a direcotry to build and given an optional output path,
// this will build the binary.
func BuildBin(path string, outputPath string) error {
	log.Printf("Building '%v'\n", path)
	cwd, _ := os.Getwd()
	if err := os.Chdir(path); err != nil {
		return err
	}
	var output []byte
	var err error
	if string(outputPath) == "" {
		output, err = exec.Command("go", "build").Output()
	} else {
		output, err = exec.Command("go", "build", "-o", outputPath).Output()
	}
	if err != nil {
		return fmt.Errorf("Error building bin '%v': %v", path, string(output))
	}
	if err := os.Chdir(cwd); err != nil {
		return err
	}
	return nil
}

// Given a gonit command, the stderr pipe and a boolean pointer on whether we
// should expect gonit to be killed this will print out anything printed to
// stderr and watch to see if the gonit process dies. If it dies unexpectedly
// this will exit the tests.
func watchGonit(gonitCmd *exec.Cmd, stderr io.ReadCloser, expectedExit *bool) {
	reader := bufio.NewReader(stderr)

	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			break
		}
		log.Printf("Gonit stderr message: %+v\n", string(line))
	}
	gonitCmd.Process.Wait()
	if !*expectedExit {
		log.Printf("Gonit process died unexpectedly.\n")
		os.Exit(2)
	}
}

// Given a config directory and whether we should be verbose in logging, this
// will start gonit, output the log messages and watch to make sure the gonit
// process doesn't die unexpectedly.
func StartGonit(
	configDir string, verbose *bool) (*exec.Cmd, io.ReadCloser, *bool, error) {
	cmd := exec.Command("./gonit", "-d", "10", "-c", configDir)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Println(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Println(err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, fmt.Errorf("Error starting gonit: %v\n", err.Error())
	}
	if *verbose {
		go io.Copy(os.Stdout, stdout)
		go io.Copy(os.Stderr, stderr)
	}
	log.Printf("Started gonit pid %v.\n", cmd.Process.Pid)
	expectedExit := false
	go watchGonit(cmd, stderr, &expectedExit)
	waitUntilRunning()
	return cmd, stdout, &expectedExit, nil
}

// This kills all processes gonit is running, then stops gonit. Path is used
// because we set custom pid file locations so the gonit client needs to know
// where the config files are for that.
func StopGonit(gonitCmd *exec.Cmd, verbose *bool, expectedExit *bool,
	path string) error {
	if err := RunGonitCmd("stop all", verbose, path); err != nil {
		return err
	}
	*expectedExit = true
	gonitCmd.Process.Kill()
	log.Printf("Killed gonit pid %v.\n", gonitCmd.Process.Pid)
	return nil
}

// Given a command, this will pipe the output to stdout/stderr if verbose is
// turned on.
func StartAndPipeOutput(cmd *exec.Cmd, verbose *bool) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	err = cmd.Start()
	if err != nil {
		return err
	}
	if *verbose {
		go io.Copy(os.Stdout, stdout)
		go io.Copy(os.Stderr, stderr)
	}
	return nil
}

// Waits for gonit to indicate that it is ready to take API requests.
func waitUntilRunning() {
	_, err := exec.Command("./gonit", "about").Output()
	if err != nil {
		log.Printf("Waiting for gonit.\n")
		waitUntilRunning()
	} else {
		log.Printf("Gonit is ready.\n")
	}
}

// Returns whether a given pid is running.
func DoesProcessExist(c *C, pid int) bool {
	err := syscall.Kill(pid, 0)
	if err != nil {
		return false
	}

	return true
}

// A helper function used by FindLogLine to set a timeout that will trigger
// FindLogLine to return false if the log line is not found within the timeout
// duration.
func setTimedOut(duration time.Duration, timedout *bool) {
	for {
		select {
		case <-time.After(duration):
			*timedout = true
			break
		}
	}
}

// Given an stdout pipe, a logline to find and a timeout string, this will
// return whether the logline was printed or not.
func FindLogLine(stdout io.ReadCloser, logline string, timeout string) bool {
	duration, err := time.ParseDuration(timeout)
	if err != nil {
		log.Printf("Invalid duration '%v'", timeout)
		return false
	}
	reader := bufio.NewReader(stdout)
	timedout := false
	go setTimedOut(duration, &timedout)
	prettifier := steno.NewJsonPrettifier(steno.EXCLUDE_NONE)
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			break
		}
		if timedout {
			log.Printf("Finding log line '%v' timed out.", logline)
			return false
		}
		record, err := prettifier.DecodeJsonLogEntry(string(line))
		if err != nil {
			// If we have an error, it's likely because the configmanager logs some
			// stuff before the steno log is told to output JSON, so we get some
			// non-JSON messages that can't be parsed.
			continue
		}
		if strings.Contains(record.Message, logline) {
			return true
		}
	}
	return false
}

// Given a gonit command string such as "start all", this will run the command.
// Path is used because we set custom pid file locations so the gonit client
// needs to know where the config files are for that.
func RunGonitCmd(command string, verbose *bool, path string) error {
	command = "-c " + path + " " + command
	log.Printf("Running command: './gonit %v'\n", command)
	cmd := exec.Command("./gonit", strings.Fields(command)...)
	if err := StartAndPipeOutput(cmd, verbose); err != nil {
		return err
	}
	cmd.Wait()
	return nil
}

func RemoveFilesWithExtension(path string, extension string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()

	files, err := dir.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, filename := range files {
		if filepath.Ext(filename) == extension {
			os.Remove(filepath.Join(path, filename))
		}
	}
	return nil
}
