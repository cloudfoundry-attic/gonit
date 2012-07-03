// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

var byteOrder = binary.LittleEndian

const DEFAULT_ENV_PATH = "PATH=/bin:/usr/bin:/sbin:/usr/sbin"

type Daemon struct {
	Name       string
	User       string
	Group      string
	Stdout     string
	Stderr     string
	Env        []string
	Dir        string
	Credential *syscall.Credential
	Pidfile    string
	Start      string
	Stop       string
	Restart    string
	Detached   bool
}

// Redirect child process fd (stdout | stderr) to a file
func (d *Daemon) Redirect(fd *io.Writer, where string) error {
	if where == "" {
		return nil
	}

	// exec package takes care of closing after fork+exec
	file, err := os.OpenFile(where, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	*fd = file

	return nil
}

// exec.Cmd wrapper
func (d *Daemon) Command(program string) (*exec.Cmd, error) {
	err := d.lookupCredentials()
	if err != nil {
		return nil, err
	}

	argv := strings.Fields(program)

	path, err := exec.LookPath(argv[0])
	if err != nil {
		return nil, err
	}

	// XXX moar defaults
	defaultEnv := []string{
		DEFAULT_ENV_PATH,
	}
	env := make([]string, len(defaultEnv)+len(d.Env))
	copy(env, defaultEnv)
	copy(env[len(defaultEnv):], d.Env)

	cmd := &exec.Cmd{
		Path: path,
		Args: argv,
		Env:  env,
		Dir:  d.Dir,
		SysProcAttr: &syscall.SysProcAttr{
			Setsid:     true,
			Credential: d.Credential,
		},
	}

	return cmd, nil
}

// Fork+Exec program with std{out,err} redirected and
// new session so program becomes the session and process group leader.
func (d *Daemon) Spawn(program string) (*exec.Cmd, error) {
	cmd, err := d.Command(program)

	if err != nil {
		return nil, err
	}

	err = d.Redirect(&cmd.Stderr, d.Stderr)
	if err != nil {
		return nil, err
	}

	err = d.Redirect(&cmd.Stdout, d.Stdout)
	if err != nil {
		return nil, err
	}

	err = cmd.Start()

	return cmd, err
}

// Start a process:
// If Detached == true; process must detached itself and
// manage its own Pidfile.
// Otherwise; we will detach the process and manage its Pidfile.
func (d *Daemon) StartProcess() (int, error) {
	if d.Detached {
		cmd, err := d.Spawn(d.Start)
		if err != nil {
			return 0, err
		}

		err = cmd.Wait()

		return cmd.Process.Pid, err
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return 0, err
	}

	defer pr.Close()
	defer pw.Close()

	ret, err := fork()
	if err != nil {
		return 0, err
	}

	if ret == 0 { // child
		pr.Close()

		status := 0

		cmd, err := d.Spawn(d.Start)
		if err == nil {
			// write pid to parent
			err = binary.Write(pw, byteOrder, int32(cmd.Process.Pid))

			cmd.Process.Release()
		}

		if err != nil {
			// propagate errno to parent via exit status
			if perr, ok := err.(*os.PathError); ok {
				status = int(perr.Err.(syscall.Errno))
				os.Exit(status)
			}
			os.Exit(1)
		}

		os.Exit(0)
	} else { // parent
		pw.Close()

		var status syscall.WaitStatus

		_, err := syscall.Wait4(int(ret), &status, 0, nil)

		if err != nil {
			return 0, err
		}

		if status.ExitStatus() != 0 {
			return -1, syscall.Errno(status.ExitStatus())
		}

		var pid int32
		// read pid from child
		err = binary.Read(pr, byteOrder, &pid)

		if err == nil {
			err = d.SavePid(int(pid))
		}

		return int(pid), err
	}

	panic("not reached") // shutup compiler
}

// Stop a process:
// Spawn Stop program if configured,
// otherwise send SIGTERM.
func (d *Daemon) StopProcess() error {
	if d.Stop == "" {
		pid, err := d.Pid()
		if err != nil {
			return err
		}
		return syscall.Kill(pid, syscall.SIGTERM)
	}

	cmd, err := d.Spawn(d.Stop)
	if err != nil {
		return err
	}

	return cmd.Wait()
}

// Restart a process:
// Spawn Restart program if configured,
// otherwise call StopProcess() + StartProcess()
func (d *Daemon) RestartProcess() error {
	if d.Restart == "" {
		err := d.StopProcess()
		if err != nil {
			return err
		}
		_, err = d.StartProcess()
		return err
	}

	cmd, err := d.Spawn(d.Restart)
	if err != nil {
		return err
	}

	return cmd.Wait()
}

// Helper method to check if process is running via Pidfile
func (d *Daemon) IsRunning() bool {
	pid, err := d.Pid()
	if err != nil {
		return false
	}

	err = syscall.Kill(pid, 0)
	if err != nil {
		return false
	}

	return true
}

// Read pid from a file
func ReadPidFile(path string) (int, error) {
	pid, err := ioutil.ReadFile(path)
	if err == nil {
		return strconv.Atoi(string(pid))
	}
	return 0, err
}

// Read pid from Pidfile
func (d *Daemon) Pid() (int, error) {
	return ReadPidFile(d.Pidfile)
}

// Write pid to a file
func WritePidFile(pid int, path string) error {
	pidString := []byte(strconv.Itoa(pid))
	err := ioutil.WriteFile(path, pidString, 0644)
	return err
}

// Write pid to Pidfile
func (d *Daemon) SavePid(pid int) error {
	return WritePidFile(pid, d.Pidfile)
}

// If User is configured, lookup and set Uid
func (d *Daemon) lookupUid() error {
	if d.User == "" {
		return nil
	}

	id, err := user.Lookup(d.User)
	if err != nil {
		return err
	}

	uid, _ := strconv.Atoi(id.Uid)
	gid, _ := strconv.Atoi(id.Gid)

	if d.Credential == nil {
		d.Credential = &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		}
	} else {
		d.Credential.Uid = uint32(uid)
	}

	return nil
}

// If Group is configured, lookup and set Gid
func (d *Daemon) lookupGid() error {
	if d.Group == "" {
		return nil
	}

	gid, err := LookupGroupId(d.Group)
	if err != nil {
		return err
	}

	if d.Credential == nil {
		d.Credential = &syscall.Credential{
			Uid: uint32(os.Getuid()),
			Gid: uint32(gid),
		}
	} else {
		d.Credential.Uid = uint32(gid)
	}

	return nil
}

func (d *Daemon) lookupCredentials() error {
	if err := d.lookupUid(); err != nil {
		return err
	}
	if err := d.lookupGid(); err != nil {
		return err
	}
	return nil
}

// go does not have a fork() wrapper w/o exec
func fork() (int, error) {
	darwin := runtime.GOOS == "darwin"

	ret, ret2, errno := syscall.RawSyscall(syscall.SYS_FORK, 0, 0, 0)
	if errno != 0 {
		return 0, errno
	}

	// see syscall/exec_bsd.go
	if darwin && ret2 == 1 {
		ret = 0
	}

	return int(ret), nil
}

// until we have user.LookupGroupId: http://codereview.appspot.com/4589049
func LookupGroupId(group string) (int, error) {
	const (
		GR_NAME = iota
		_
		GR_ID
	)

	file, err := os.Open("/etc/group")
	if err != nil {
		return -1, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)

	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}

		line = bytes.TrimSpace(line)

		if len(line) == 0 || line[0] == '#' {
			continue
		}

		fields := strings.Split(string(line), ":")
		if len(fields) == 4 && fields[GR_NAME] == group {
			return strconv.Atoi(fields[GR_ID])
		}
	}

	return -1, errors.New("group: unknown group " + group)
}
