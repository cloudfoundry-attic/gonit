// Copyright (c) 2012 VMware, Inc.

// general purpose program for test process lifecycle

package main

import (
	"flag"
	"fmt"
	"github.com/cloudfoundry/gonit"
	"github.com/cloudfoundry/gonit/test/helper"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var (
	fork       = flag.Bool("F", false, "Fork me")
	grand      = flag.Bool("G", false, "Behave as grandchild process")
	memballoon = flag.Bool("MB", false, "balloon memory used")
	dir        = flag.String("d", os.TempDir(), "test directory")
	name       = flag.String("n", "test", "process name")
	pidfile    = flag.String("p", "test.pid", "process pid file")
	sleep      = flag.String("s", "10s", "sleep duration")
	wait       = flag.String("w", "", "start/stop wait duration")
	exit       = flag.Int("x", 0, "exit code")
)

var restarts = 0

func saveProcessInfo() {
	file := filepath.Join(*dir, *name+".json")
	info := helper.CurrentProcessInfo()
	info.Restarts = restarts
	helper.WriteData(info, file)
}

func savePid() {
	err := gonit.WritePidFile(os.Getpid(), *pidfile)
	if err != nil {
		log.Fatal(err)
	}
}

func handleSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)

	for {
		select {
		case s := <-c:
			if s == syscall.SIGHUP {
				restarts++
				saveProcessInfo()
			}
		}
	}
}

// we can't just fork() since child process does not inherit threads,
// which are required for signal handlers.
// so, fork+exec with the same args, but change -F fork flag to -G
func forkme() {
	args := os.Args
	for i, arg := range args {
		if arg == "-F" {
			args[i] = "-G"
			break
		}
	}

	cmd := &exec.Cmd{
		Path: args[0],
		Args: args,
		Dir:  *dir,
		SysProcAttr: &syscall.SysProcAttr{
			Setsid: true,
		},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	err := cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	cmd.Process.Release()
	os.Exit(0)
}

func balloon() {
	dess := make([][]float64, 300)
	for i := 0; i < 300; i++ {
		fmt.Fprintf(os.Stdout, "Ballooning %v\n", i)
		dess[i] = make([]float64, 900000)
		time.Sleep(time.Duration(100) * time.Millisecond)
	}
}

func sleepDuration(name string, value string) time.Duration {
	if value == "" {
		return time.Duration(0)
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		log.Fatalf("Invalid %s '%s': %v", name, value, err)
	}
	return duration
}

func main() {
	flag.Parse()
	log.SetFlags(log.Ltime | log.Lshortfile)

	pause := sleepDuration("-s", *sleep)
	waitTime := sleepDuration("-w", *wait)

	cmd := flag.Args()[0]
	switch cmd {
	case "start":
		if *fork {
			forkme()
		}
		if *grand {
			if *wait != "" {
				fmt.Fprintf(os.Stdout, "Start (savePid) wait=%s\n", *wait)
				time.Sleep(waitTime)
			}
			savePid()
			go handleSignals()
		} else {
			savePid()
		}

		saveProcessInfo()
		fmt.Fprintf(os.Stdout, "Started. [sleep(%s)]\n", *sleep)
		if *memballoon {
			balloon()
		} else {
			time.Sleep(pause)
		}

		fmt.Fprintf(os.Stdout, "Stopped. [exit(%d)]\n", *exit)
		os.Exit(*exit)
	case "stop":
		if *wait != "" {
			fmt.Fprintf(os.Stdout, "Stop wait=%s\n", *wait)
			time.Sleep(waitTime)
		}
		pid, err := gonit.ReadPidFile(*pidfile)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(os.Stdout, "Sending SIGTERM to pid=%d\n", pid)
		err = syscall.Kill(pid, syscall.SIGTERM)
		if err != nil {
			log.Fatal(err)
		}
	case "restart":
		pid, err := gonit.ReadPidFile(*pidfile)
		if err != nil {
			log.Fatal(err)
		}
		err = syscall.Kill(pid, syscall.SIGHUP)
		if err != nil {
			log.Fatal(err)
		}
	case "status":
		pid, err := gonit.ReadPidFile(*pidfile)
		if err != nil {
			log.Fatal(err)
		}
		err = syscall.Kill(pid, 0)
		if err == nil {
			log.Printf("Process is alive (pid=%d)", pid)
		} else {
			log.Fatal("Process is dead")
		}
	default:
		log.Fatalf("Unsupported command `%s'", cmd)
	}
}
