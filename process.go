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

// Redirect child process fd (stdout | stderr) to a file
func (p *Process) Redirect(fd *io.Writer, where string) error {
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
func (p *Process) Command(program string) (*exec.Cmd, error) {
	var credential *syscall.Credential

	if p.Uid != "" || p.Gid != "" {
		credential = &syscall.Credential{}
		err := p.lookupCredentials(credential)
		if err != nil {
			return nil, err
		}
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
	env := make([]string, len(defaultEnv)+len(p.Env))
	copy(env, defaultEnv)
	copy(env[len(defaultEnv):], p.Env)

	cmd := &exec.Cmd{
		Path: path,
		Args: argv,
		Env:  env,
		Dir:  p.Dir,
		SysProcAttr: &syscall.SysProcAttr{
			Setsid:     true,
			Credential: credential,
		},
	}

	return cmd, nil
}

// Fork+Exec program with std{out,err} redirected and
// new session so program becomes the session and process group leader.
func (p *Process) Spawn(program string) (*exec.Cmd, error) {
	cmd, err := p.Command(program)

	if err != nil {
		return nil, err
	}

	err = p.Redirect(&cmd.Stderr, p.Stderr)
	if err != nil {
		return nil, err
	}

	err = p.Redirect(&cmd.Stdout, p.Stdout)
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
func (p *Process) StartProcess() (int, error) {
	if p.Detached {
		cmd, err := p.Spawn(p.Start)
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

		cmd, err := p.Spawn(p.Start)
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
			err = p.SavePid(int(pid))
		}

		return int(pid), err
	}

	panic("not reached") // shutup compiler
}

// Stop a process:
// Spawn Stop program if configured,
// otherwise send SIGTERM.
func (p *Process) StopProcess() error {
	if p.Stop == "" {
		pid, err := p.Pid()
		if err != nil {
			return err
		}
		return syscall.Kill(pid, syscall.SIGTERM)
	}

	cmd, err := p.Spawn(p.Stop)
	if err != nil {
		return err
	}

	return cmd.Wait()
}

// Restart a process:
// Spawn Restart program if configured,
// otherwise call StopProcess() + StartProcess()
func (p *Process) RestartProcess() error {
	if p.Restart == "" {
		err := p.StopProcess()
		if err != nil {
			return err
		}
		_, err = p.StartProcess()
		return err
	}

	cmd, err := p.Spawn(p.Restart)
	if err != nil {
		return err
	}

	return cmd.Wait()
}

// Helper method to check if process is running via Pidfile
func (p *Process) IsRunning() bool {
	pid, err := p.Pid()
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
func (p *Process) Pid() (int, error) {
	return ReadPidFile(p.Pidfile)
}

// Write pid to a file
func WritePidFile(pid int, path string) error {
	pidString := []byte(strconv.Itoa(pid))
	err := ioutil.WriteFile(path, pidString, 0644)
	return err
}

// Write pid to Pidfile
func (p *Process) SavePid(pid int) error {
	return WritePidFile(pid, p.Pidfile)
}

// If User is configured, lookup and set Uid
func (p *Process) lookupUid(credential *syscall.Credential) error {
	if p.Uid == "" {
		return nil
	}

	id, err := user.Lookup(p.Uid)
	if err != nil {
		return err
	}

	uid, _ := strconv.Atoi(id.Uid)
	gid, _ := strconv.Atoi(id.Gid)

	credential.Uid = uint32(uid)

	if p.Gid == "" {
		credential.Gid = uint32(gid)
	}

	return nil
}

// If Group is configured, lookup and set Gid
func (p *Process) lookupGid(credential *syscall.Credential) error {
	if p.Gid == "" {
		return nil
	}

	gid, err := LookupGroupId(p.Gid)
	if err != nil {
		return err
	}

	credential.Gid = uint32(gid)

	if p.Uid == "" {
		credential.Uid = uint32(os.Getuid())
	}

	return nil
}

func (p *Process) lookupCredentials(credential *syscall.Credential) error {
	if err := p.lookupUid(credential); err != nil {
		return err
	}
	if err := p.lookupGid(credential); err != nil {
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
