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

var (
	byteOrder = binary.LittleEndian
)

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
func (self *Daemon) Redirect(fd *io.Writer, where string) error {
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
func (self *Daemon) Command(program string) (*exec.Cmd, error) {
	err := self.lookupCredentials()
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
		"PATH=/bin:/usr/bin",
	}
	env := make([]string, 0, len(defaultEnv)+len(self.Env))
	for _, p := range defaultEnv {
		env = append(env, p)
	}
	for _, p := range self.Env {
		env = append(env, p)
	}

	cmd := &exec.Cmd{
		Path: path,
		Args: argv,
		Env:  env,
		Dir:  self.Dir,
		SysProcAttr: &syscall.SysProcAttr{
			Setsid:     true,
			Credential: self.Credential,
		},
	}

	return cmd, nil
}

// Fork+Exec program with std{out,err} redirected and
// new session so program becomes the session and process group leader.
func (self *Daemon) Spawn(program string) (*exec.Cmd, error) {
	cmd, err := self.Command(program)

	if err != nil {
		return nil, err
	}

	err = self.Redirect(&cmd.Stderr, self.Stderr)
	if err != nil {
		return nil, err
	}

	err = self.Redirect(&cmd.Stdout, self.Stdout)
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
func (self *Daemon) StartProcess() (int, error) {
	if self.Detached {
		cmd, err := self.Spawn(self.Start)
		if err != nil {
			return 0, err
		}

		err = cmd.Wait()

		return cmd.Process.Pid, err
	} else {
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

			cmd, err := self.Spawn(self.Start)
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
				err = self.SavePid(int(pid))
			}

			return int(pid), err
		}
	}

	panic("notreached") // shutup compiler
}

// Stop a process:
// Spawn Stop program if configured,
// otherwise send SIGTERM.
func (self *Daemon) StopProcess() error {
	if self.Stop == "" {
		pid, err := self.Pid()
		if err != nil {
			return err
		}
		return syscall.Kill(pid, syscall.SIGTERM)
	}

	cmd, err := self.Spawn(self.Stop)
	if err != nil {
		return err
	}

	return cmd.Wait()
}

// Restart a process:
// Spawn Restart program if configured,
// otherwise call StopProcess() + StartProcess()
func (self *Daemon) RestartProcess() error {
	if self.Restart == "" {
		err := self.StopProcess()
		if err != nil {
			return err
		}
		_, err = self.StartProcess()
		return err
	}

	cmd, err := self.Spawn(self.Restart)
	if err != nil {
		return err
	}

	return cmd.Wait()
}

// Helper method to check if process is running via Pidfile
func (self *Daemon) IsRunning() bool {
	pid, err := self.Pid()
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
func (self *Daemon) Pid() (int, error) {
	return ReadPidFile(self.Pidfile)
}

// Write pid to a file
func WritePidFile(pid int, path string) error {
	pidString := []byte(strconv.Itoa(pid))
	err := ioutil.WriteFile(path, pidString, 0644)
	return err
}

// Write pid to Pidfile
func (self *Daemon) SavePid(pid int) error {
	return WritePidFile(pid, self.Pidfile)
}

// If User is configured, lookup and set Uid
func (self *Daemon) lookupUid() error {
	if self.User == "" {
		return nil
	}

	id, err := user.Lookup(self.User)
	if err != nil {
		return err
	}

	uid, _ := strconv.Atoi(id.Uid)
	gid, _ := strconv.Atoi(id.Gid)

	if self.Credential == nil {
		self.Credential = &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		}
	} else {
		self.Credential.Uid = uint32(uid)
	}

	return nil
}

// If Group is configured, lookup and set Gid
func (self *Daemon) lookupGid() error {
	if self.Group == "" {
		return nil
	}

	gid, err := LookupGroupId(self.Group)
	if err != nil {
		return err
	}

	if self.Credential == nil {
		self.Credential = &syscall.Credential{
			Uid: uint32(os.Getuid()),
			Gid: uint32(gid),
		}
	} else {
		self.Credential.Uid = uint32(gid)
	}

	return nil
}

func (self *Daemon) lookupCredentials() error {
	if err := self.lookupUid(); err != nil {
		return err
	}
	if err := self.lookupGid(); err != nil {
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
