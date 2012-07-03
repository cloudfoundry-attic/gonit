// Copyright (c) 2012 VMware, Inc.

package helper

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
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
}

func CurrentProcessInfo() *ProcessInfo {
	cwd, _ := os.Getwd()
	grp, _ := os.Getgroups()
	// no syscall.Getsid() wrapper on Linux?
	sid, _, _ := syscall.RawSyscall(syscall.SYS_GETSID, 0, 0, 0)

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
