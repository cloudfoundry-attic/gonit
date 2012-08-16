// Copyright (c) 2012 VMware, Inc.

package main

import (
	"flag"
	"fmt"
	"github.com/cloudfoundry/gonit"
	"log"
	"net/rpc"
	"net/rpc/jsonrpc"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var (
	// flags
	config     string
	pidfile    string
	rpcUrl     string
	polltime   time.Duration
	group      bool
	foreground bool
	version    bool

	// defaults
	home           = os.Getenv("HOME")
	name           = filepath.Base(os.Args[0]) // gonit
	defaultPidFile = filepath.Join(home, "."+name+".pid")
	defaultRpcUrl  = filepath.Join(home, "."+name+".sock")

	// internal
	api       *gonit.API
	rpcServer *gonit.RpcServer
)

func main() {
	parseFlags()

	if version {
		showVersion()
		return
	}

	configManager := &gonit.ConfigManager{}

	if config != "" {
		err := configManager.Parse(config)
		if err != nil {
			log.Fatal(err)
		}
	}

	api = gonit.NewAPI(configManager)

	args := flag.Args()
	if len(args) == 0 {
		if polltime != 0 {
			runDaemon()
		} else {
			log.Fatal("Nothing todo (yet)")
		}
	} else {
		// example: gonit stop all
		cmd := args[0]
		var arg string
		if len(args) == 2 {
			arg = args[1]
		}
		runCommand(cmd, arg)
	}
}

func parseFlags() {
	flag.BoolVar(&version, "V", false, "Print version number")
	flag.BoolVar(&group, "g", false, "Use process group")
	flag.BoolVar(&foreground, "I", false, "Do not run in background")
	flag.StringVar(&config, "c", "", "Config path")
	// XXX should be able to use gonit.yml for the following opts
	flag.StringVar(&pidfile, "p", defaultPidFile, "Pid file path")
	flag.StringVar(&rpcUrl, "s", defaultRpcUrl, "RPC server URL")
	flag.DurationVar(&polltime, "d", 0, "Run as a daemon with duration")

	flag.Parse()
}

func rpcClient() *rpc.Client {
	url, err := url.Parse(rpcUrl)
	if err != nil {
		log.Fatal(err)
	}

	network := url.Scheme
	if network == "" {
		network = "unix"
	}

	var address string
	if network == "unix" {
		address = url.Path
	} else {
		address = url.Host
	}

	client, err := jsonrpc.Dial(network, address)
	if err != nil {
		log.Fatal(err)
	}

	return client
}

func runCommand(cmd, arg string) {
	var client gonit.CliClient

	if isRunning() {
		rpc := rpcClient()
		defer rpc.Close()
		client = gonit.NewRemoteClient(rpc, api)
	} else {
		client = gonit.NewLocalClient(api)
	}

	method, name := gonit.RpcArgs(cmd, arg, group)

	reply, err := client.Call(method, name)

	log.Printf("%#v", reply) // XXX make perty
	if err != nil {
		log.Fatal(err)
	}
}

func reload() {
	log.Printf("XXX reload config")
}

func wakeup() {
	log.Printf("XXX wakeup")
}

func shutdown() {
	log.Printf("Quit")

	if rpcServer != nil {
		rpcServer.Shutdown()
	}

	os.Exit(0)
}

func start() {
	var err error

	rpcServer, err = gonit.NewRpcServer(rpcUrl)
	if err != nil {
		log.Fatal(err)
	}

	rpc.Register(api)

	go rpcServer.Serve()
}

var handlers = map[syscall.Signal]func(){
	syscall.SIGTERM: shutdown,
	syscall.SIGINT:  shutdown,
	syscall.SIGHUP:  reload,
	syscall.SIGUSR1: wakeup,
}

func loop() {
	sigchan := make(chan os.Signal, 1)
	for sig, _ := range handlers {
		signal.Notify(sigchan, sig)
	}

	for {
		select {
		case signal := <-sigchan:
			sig, _ := signal.(syscall.Signal)
			if handler, ok := handlers[sig]; ok {
				handler()
			}
		}
	}
}

func isRunning() bool {
	pid, err := gonit.ReadPidFile(pidfile)

	return err == nil && syscall.Kill(pid, 0) == nil
}

func runDaemon() {
	if isRunning() {
		log.Fatalf("%s daemon is already running", name)
	}

	if !foreground {
		log.Print("daemonize - not yet supported")
	}

	log.Printf("Saving %s daemon pid to file=%s", name, pidfile)
	err := gonit.WritePidFile(os.Getpid(), pidfile)
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(pidfile)

	start()
	loop()
}

func showVersion() {
	fmt.Printf("Gonit version %s\n", gonit.VERSION)
}
