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
)

var (
	// flags
	config     string
	pidfile    string
	rpcUrl     string
	poll       int
	group      bool
	foreground bool
	version    bool

	// internal
	api          *gonit.API
	rpcServer    *gonit.RpcServer
	eventMonitor *gonit.EventMonitor
	settings     *gonit.Settings
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
	} else {
		configManager.ApplyDefaultSettings()
	}

	settings = configManager.Settings
	applySettings()

	api = gonit.NewAPI(configManager)
	args := flag.Args()
	if len(args) == 0 {
		if settings.PollInterval != 0 {
			runDaemon(api.Control, configManager)
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

func applySettings() {
	if pidfile != "" {
		settings.Process.Pidfile = pidfile
	}
	if rpcUrl != "" {
		settings.RpcServer = rpcUrl
	}
	if poll != 0 {
		settings.PollInterval = poll
	}
}

func parseFlags() {
	flag.BoolVar(&version, "V", false, "Print version number")
	flag.BoolVar(&group, "g", false, "Use process group")
	flag.BoolVar(&foreground, "I", false, "Do not run in background")
	flag.StringVar(&config, "c", "", "Config path")
	flag.StringVar(&pidfile, "p", "", "Pid file path")
	flag.StringVar(&rpcUrl, "s", "", "RPC server URLq")
	flag.IntVar(&poll, "d", 0, "Run as a daemon with duration")

	const named = "the named process or group"
	const all = "all processes"

	actions := []struct {
		usage       string
		description string
		what        string
	}{
		{"start all", "Start", all},
		{"start name", "Only start", named},
		{"stop all", "Stop", all},
		{"stop name", "Only stop", named},
		{"restart all", "Restart", all},
		{"restart name", "Only restart", named},
		{"monitor all", "Enable monitoring for", all},
		{"monitor name", "Only enable monitoring of", named},
		{"unmonitor all", "Disable monitoring for", all},
		{"unmonitor name", "Only disable monitoring of", named},
		{"status all", "Print full status info for", all},
		{"status name", "Only print short status info for", named},
		{"summary", "Print short status information for", all},
	}

	flag.Usage = func() {
		name := filepath.Base(os.Args[0]) // gonit
		fmt.Println("Usage:", name, "[options] {arguments}")

		fmt.Println("Options are as follows:")
		flag.PrintDefaults()

		fmt.Println("Optional action arguments are as follows:")
		for _, action := range actions {
			fmt.Printf("  %-20s - %s %s\n", action.usage,
				action.description, action.what)
		}
	}

	flag.Parse()
}

func rpcClient() *rpc.Client {
	url, err := url.Parse(settings.RpcServer)
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

	if settings.Process.IsRunning() {
		rpc := rpcClient()
		defer rpc.Close()
		client = gonit.NewRemoteClient(rpc, api)
	} else {
		client = gonit.NewLocalClient(api)
	}

	method, name := gonit.RpcArgs(cmd, arg, group)

	reply, err := client.Call(method, name)

	if err != nil {
		log.Fatal(err)
	}

	if formatter, ok := reply.(gonit.CliFormatter); ok {
		formatter.Print(os.Stdout)
	} else {
		log.Printf("%#v", reply) // TODO make perty
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
	eventMonitor.Stop()
	if rpcServer != nil {
		rpcServer.Shutdown()
	}

	os.Exit(0)
}

func start() {
	var err error

	rpcServer, err = gonit.NewRpcServer(settings.RpcServer)
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

func runDaemon(control *gonit.Control, configManager *gonit.ConfigManager) {
	process := settings.Process
	if process.IsRunning() {
		log.Fatalf("%s daemon is already running", process.Name)
	}

	if !foreground {
		log.Print("daemonize - not yet supported")
	}

	err := process.SavePid(os.Getpid())
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(process.Pidfile)
	createEventMonitor(control, configManager)
	start()
	loop()
}

func createEventMonitor(control *gonit.Control,
	configManager *gonit.ConfigManager) {
	eventMonitor = &gonit.EventMonitor{}
	err := eventMonitor.Start(configManager, control)
	if err != nil {
		log.Fatal(err)
	}
	control.RegisterEventMonitor(eventMonitor)
}

func showVersion() {
	fmt.Printf("Gonit version %s\n", gonit.VERSION)
}
