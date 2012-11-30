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
	"launchpad.net/goyaml"
	"log"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
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

var TestProcess, goprocess, toplevel string
var MAX_GONIT_RETRIES int = 10

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
		log.Panic(err)
	}
	return json
}

func WriteData(data interface{}, file string) {
	err := ioutil.WriteFile(file, toJson(data), 0666)
	if err != nil {
		log.Panicf("WriteFile(%s): %v", file, err)
	}
}

func ReadData(data interface{}, file string) {
	contents, err := ioutil.ReadFile(file)
	if err != nil {
		log.Panic(err)
	}

	err = json.Unmarshal(contents, data)
	if err != nil {
		log.Panic(err)
	}
}

func TouchFile(name string, mode os.FileMode) {
	dst, err := os.Create(name)
	if err != nil {
		log.Panic(err)
	}
	dst.Close()
	err = os.Chmod(name, mode)
	if err != nil {
		log.Panic(err)
	}
}

func CopyFile(srcName, dstName string, mode os.FileMode) {
	src, err := os.Open(srcName)
	if err != nil {
		log.Panic(err)
	}
	defer src.Close()

	dst, err := os.Create(dstName)
	if err != nil {
		log.Panic(err)
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		log.Panic(err)
	}

	err = os.Chmod(dstName, mode)
	if err != nil {
		log.Panic(err)
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
		log.Panic(err)
	}
	os.Chmod(dir, 0755) // rx perms for all

	if goprocess == "" {
		// binary used for the majority of tests
		goprocess = path.Dir(os.Args[0]) + "/goprocess"
		err := BuildBin("test/process", goprocess)
		if err != nil {
			log.Panic(err)
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
		"-p", pidfile,
	}

	for _, arg := range flags {
		args = append(args, arg)
	}

	if detached {
		// process will detach itself
		args = append(args, "-F")

		// configure stop + restart commands with
		// the same flags as the start command
		stop = mkcmd(args, "stop")
		restart = mkcmd(args, "restart")
	}

	start = mkcmd(args, "start")
	return &Process{
		Name:    name,
		Start:   strings.Join(start, " "),
		Stop:    strings.Join(stop, " "),
		Restart: strings.Join(restart, " "),
		Dir:     dir,
		Stderr:  logfile,
		Stdout:  logfile,
		Pidfile: pidfile,
	}
}

func CreateProcessGroupCfg(name string, dir string, pg *ProcessGroup) error {
	yaml, err := goyaml.Marshal(pg)
	if err != nil {
		return err
	}

	file := filepath.Join(dir, name+"-gonit.yml")
	if err := ioutil.WriteFile(file, yaml, 0666); err != nil {
		return err
	}

	return nil
}

func CreateGonitCfg(numProcesses int, pname string, writePath string,
	procPath string, includeEvents bool) error {
	pg := &ProcessGroup{}
	processes := map[string]*Process{}
	for i := 0; i < numProcesses; i++ {
		procName := pname + strconv.Itoa(i)
		pidfile := fmt.Sprintf("%v/%v.pid", writePath, procName)
		runCmd := fmt.Sprintf("%v -d %v -n %v -p %v", procPath, writePath, procName,
			pidfile)
		if includeEvents {
			runCmd += " -MB"
		}
		process := &Process{
			Name:        procName,
			Description: "Test process " + procName,
			Start:       runCmd + " start",
			Stop:        runCmd + " stop",
			Restart:     runCmd + " restart",
			Pidfile:     pidfile,
		}
		if includeEvents {
			process.Actions = map[string][]string{}
			process.Actions["alert"] = []string{"memory_over_1"}
			process.Actions["restart"] = []string{"memory_over_6"}
			memoryOver1 := &Event{
				Name:        "memory_over_1",
				Description: "The memory for a process is too high",
				Rule:        "memory_used > 1mb",
				Interval:    "1s",
				Duration:    "1s",
			}
			memoryOver6 := &Event{
				Name:        "memory_over_6",
				Description: "The memory for a process is too high",
				Rule:        "memory_used > 6mb",
				Interval:    "1s",
				Duration:    "1s",
			}
			events := map[string]*Event{}
			events["memory_over_1"] = memoryOver1
			events["memory_over_6"] = memoryOver6
			pg.Events = events
		}
		processes[procName] = process
	}
	pg.Processes = processes
	return CreateProcessGroupCfg(pname, writePath, pg)
}

func CreateGonitSettings(gonitPidfile string, gonitDir string, procDir string) *Settings {
	logging := &LoggerConfig{
		Codec: "json",
		Level: "debug",
	}
	settings := &Settings{Logging: logging}
	daemon := &Process{
		Pidfile: gonitPidfile,
		Dir:     gonitDir,
		Name:    "gonit",
	}
	settings.Daemon = daemon
	settings.ApplyDefaults()
	yaml, _ := goyaml.Marshal(settings)
	err := ioutil.WriteFile(procDir+"/gonit.yml", yaml, 0666)
	if err != nil {
		log.Panicf("WriteFile(%s): %v", procDir+"/gonit.yml", err)
	}
	return settings
}

// Read pid from a file.
func ProxyReadPidFile(path string) (int, error) {
	return ReadPidFile(path)
}

func findPath(path string, name string) (string, error) {
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(path, name)); err == nil {
		return path, nil
	}
	return findPath(filepath.Join(path, ".."), name)
}

func topLevel() string {
	if toplevel == "" {
		dir, err := findPath(".", ".git")
		if err != nil {
			log.Panic(err)
		}
		toplevel = dir
	}
	return toplevel
}

// Given the path to a direcotry to build and given an optional output path,
// this will build the binary.
func BuildBin(path string, outputPath string) error {
	var output []byte
	var err error
	if !filepath.IsAbs(path) {
		path = filepath.Join(topLevel(), path)
	}
	path = filepath.Join(path, "main.go")
	log.Printf("Building '%v'", path)
	output, err =
		exec.Command("go", "build", "-o", outputPath, path).Output()
	if err != nil {
		return fmt.Errorf("Error building bin '%v': %v", path, string(output))
	}
	return nil
}

// Given a gonit command, prints out any stderr messages.
func printCmdStderr(gonitCmd *exec.Cmd, stderr io.ReadCloser) {
	reader := bufio.NewReader(stderr)
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			break
		}
		log.Printf("Gonit stderr message: %+v", string(line))
	}
	gonitCmd.Process.Wait()
}

// Given a config directory, this will start gonit, output the log messages and
// watch gonit to print out error messages.
func StartGonit(configDir string) (*exec.Cmd, io.ReadCloser, error) {
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
		return nil, nil, fmt.Errorf("Error starting gonit: %v", err.Error())
	}
	log.Printf("Started gonit pid %v.", cmd.Process.Pid)
	go printCmdStderr(cmd, stderr)
	err = waitUntilRunning(configDir)
	return cmd, stdout, err
}

// This kills all processes gonit is running, then stops gonit. Path is used
// because we set custom pid file locations so the gonit client needs to know
// where the config files are for that.
func StopGonit(gonitCmd *exec.Cmd, path string) error {
	pid := gonitCmd.Process.Pid
	if !DoesProcessExist(pid) {
		return fmt.Errorf("Gonit process died unexpectedly.")
	}
	if err := RunGonitCmd("stop all", path); err != nil {
		return err
	}
	gonitCmd.Process.Kill()
	log.Printf("Killed gonit pid %v.", pid)
	return nil
}

// Waits for gonit to indicate that it is ready to take API requests.
func waitUntilRunning(path string) error {
	for i := 0; i < MAX_GONIT_RETRIES; i++ {
		log.Printf("Waiting for gonit.")
		if err := RunGonitCmd("about", path); err == nil {
			log.Printf("Gonit is ready.")
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("Could not connect to gonit.")
}

// Returns whether a given pid is running.
func DoesProcessExist(pid int) bool {
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
		// TODO(lisbakke): It is possible we won't get here if ReadLine hangs.
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
func RunGonitCmd(command string, path string) error {
	command = "-c " + path + " " + command
	log.Printf("Running command: './gonit %v'", command)
	cmd := exec.Command("./gonit", strings.Fields(command)...)
	return cmd.Run()
}
