// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"fmt"
	"io/ioutil"
	"launchpad.net/goyaml"
	"os"
	"path/filepath"
	"strings"
)

// TODO:
// - Global interval for collecting resources.
// - Throw an error if the interval * MAX_DATA_TO_STORE < a rule's duration e.g.
//       if interval is 1s and MAX_DATA_TO_STORE = 120 and someone wants 3
//       minutes duration in a rule.

type ConfigManager struct {
	ProcessGroups map[string]*ProcessGroup
	Settings      *Settings
	path          string
}

type Settings struct {
	AlertTransport      string
	SocketFile          string
	RpcServerUrl        string
	ProcessPollInterval int
	Daemon              *Process
	PersistFile         string
	Logging             *LoggerConfig
}

type ProcessGroup struct {
	Name      string
	Events    map[string]*Event
	Processes map[string]*Process
}

type Event struct {
	Name        string
	Description string
	Rule        string
	Duration    string
	Interval    string
}

type Action struct {
	Name   string
	Events []string
}

type Process struct {
	Name        string
	Pidfile     string
	Start       string
	Stop        string
	Restart     string
	Gid         string
	Uid         string
	Stdout      string
	Stderr      string
	Env         []string
	Dir         string
	Description string
	DependsOn   []string
	Actions     map[string][]string
	MonitorMode string
}

const (
	CONFIG_FILE_POSTFIX   = "-gonit.yml"
	SETTINGS_FILENAME     = "gonit.yml"
	UNIX_SOCKET_TRANSPORT = "unix_socket"
	MONITOR_MODE_ACTIVE   = "active"
	MONITOR_MODE_PASSIVE  = "passive"
	MONITOR_MODE_MANUAL   = "manual"
)

const (
	DEFAULT_ALERT_TRANSPORT = "none"
)

// Given an action string name, returns the events associated with it.
func (pg *ProcessGroup) EventByName(eventName string) *Event {
	event, hasKey := pg.Events[eventName]
	if hasKey {
		return event
	}
	return nil
}

// Given a process name, returns the Process and whether it exists.
func (pg *ProcessGroup) processFromName(name string) (*Process, bool) {
	serv, hasKey := pg.Processes[name]
	return serv, hasKey
}

// For some of the maps, we want the map key name to be inside of the object so
// that it's easier to access.
func (c *ConfigManager) fillInNames() {
	for groupName, processGroup := range c.ProcessGroups {
		processGroup.Name = groupName
		for name, process := range processGroup.Processes {
			process.Name = name
			processGroup.Processes[name] = process
		}
		for name, event := range processGroup.Events {
			event.Name = name
			processGroup.Events[name] = event
		}
		c.ProcessGroups[groupName] = processGroup
	}
}

// Parses a config file into a ProcessGroup.
func (c *ConfigManager) parseConfigFile(path string) (*ProcessGroup, error) {
	processGroup := &ProcessGroup{}
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := goyaml.Unmarshal(b, processGroup); err != nil {
		return nil, err
	}
	Log.Infof("Loaded config file '%+v'", path)
	return processGroup, nil
}

// Parses a settings file into a Settings struct.
func (c *ConfigManager) parseSettingsFile(path string) (*Settings, error) {
	settings := &Settings{}
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := goyaml.Unmarshal(b, settings); err != nil {
		return nil, err
	}
	Log.Infof("Loaded settings file: '%+v'", path)
	return settings, nil
}

// Given a filename, removes -gonit.yml.
func getGroupName(filename string) string {
	// -10 because of "-gonit.yml"
	return filename[:len(filename)-10]
}

// Parses a directory for gonit files.
func (c *ConfigManager) parseDir(dirPath string) error {
	dir, err := os.Open(dirPath)
	if err != nil {
		return err
	}
	defer dir.Close()

	dirNames, err := dir.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, filename := range dirNames {
		if err := c.parseFile(filepath.Join(dirPath, filename)); err != nil {
			return err
		}
	}
	return nil
}

// Applies default global settings if some options haven't been specified.
func (c *ConfigManager) ApplyDefaultSettings() {
	if c.Settings == nil {
		c.Settings = &Settings{}
	}
	c.Settings.ApplyDefaults()
}

func (settings *Settings) ApplyDefaults() {
	if settings.AlertTransport == "" {
		settings.AlertTransport = DEFAULT_ALERT_TRANSPORT
	}
	if settings.Logging == nil {
		settings.Logging = &LoggerConfig{}
	}
	if settings.Daemon == nil {
		settings.Daemon = &Process{}
	}
	daemon := settings.Daemon
	if daemon.Dir == "" {
		daemon.Dir = os.Getenv("HOME")
	}

	if daemon.Name == "" {
		daemon.Name = filepath.Base(os.Args[0])
	}
	if daemon.Pidfile == "" {
		defaultPath := "." + daemon.Name + ".pid"
		daemon.Pidfile = filepath.Join(daemon.Dir, defaultPath)
	}

	if settings.RpcServerUrl == "" {
		defaultPath := "." + daemon.Name + ".sock"
		settings.RpcServerUrl = filepath.Join(daemon.Dir, defaultPath)
	}

	if settings.PersistFile == "" {
		settings.PersistFile = filepath.Join(daemon.Dir, ".gonit.persist.yml")
	}
}

func (s *Settings) validatePersistFile() error {
	_, err := os.Stat(s.PersistFile)
	if err != nil {
		// The file doesn't exist. See if we can create it.
		if file, err := os.Create(s.PersistFile); err != nil {
			return err
		} else {
			file.Close()
			os.Remove(s.PersistFile)
		}
	}
	return nil
}

// Parses a file.
func (c *ConfigManager) parseFile(path string) error {
	_, filename := filepath.Split(path)
	var err error
	if filename == SETTINGS_FILENAME {
		if c.Settings, err = c.parseSettingsFile(path); err != nil {
			return err
		}
	} else if strings.HasSuffix(filename, CONFIG_FILE_POSTFIX) {
		groupName := getGroupName(filename)
		c.ProcessGroups[groupName], err = c.parseConfigFile(path)
		if err != nil {
			return err
		}
	}
	return nil
}

// Main function to call, parses a path for gonit config file(s).
func (c *ConfigManager) LoadConfig(path string) error {
	c.path = path
	if path == "" {
		return fmt.Errorf("No config given.")
	}

	c.ProcessGroups = map[string]*ProcessGroup{}
	c.Settings = &Settings{}
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("Error stating path '%+v'.", path)
	}
	if fileInfo.IsDir() {
		if err = c.parseDir(path); err != nil {
			return err
		}
	} else {
		if err := c.parseFile(path); err != nil {
			return err
		}
	}
	c.fillInNames()
	if (*c.Settings == Settings{}) {
		Log.Info("No settings found, using defaults")
	}
	c.ApplyDefaultSettings()
	c.applyDefaultConfigOpts()
	if err := c.validate(); err != nil {
		return err
	}
	return nil
}

func (c *ConfigManager) applyDefaultMonitorMode() {
	for _, pg := range c.ProcessGroups {
		for _, process := range pg.Processes {
			if process.MonitorMode == "" {
				process.MonitorMode = MONITOR_MODE_ACTIVE
			}
		}
	}
}

func (c *ConfigManager) applyDefaultConfigOpts() {
	c.applyDefaultMonitorMode()
}

// Validates that certain fields exist in the config file.
func (pg ProcessGroup) validateRequiredFieldsExist() error {
	for name, process := range pg.Processes {
		if process.Name == "" || process.Description == "" ||
			process.Pidfile == "" || process.Start == "" {
			return fmt.Errorf("%v must have name, description, pidfile and start.",
				name)
		}
	}
	for name, event := range pg.Events {
		if event.Name == "" || event.Description == "" || event.Rule == "" {
			return fmt.Errorf("%v must have name, description, rule, and "+
				"actions.", name)
		}
	}
	return nil
}

// Validates various links in a config.
func (pg *ProcessGroup) validateLinks() error {
	// TODO: Validate the event links.
	for _, process := range pg.Processes {
		for _, dependsOnName := range process.DependsOn {
			if _, hasKey := pg.processFromName(dependsOnName); hasKey == false {
				return fmt.Errorf("Process %v has an unknown dependson '%v'.",
					process.Name, dependsOnName)
			}
		}
	}

	return nil
}

// Valitades settings.
func (s *Settings) validate() error {
	if s.AlertTransport == UNIX_SOCKET_TRANSPORT && s.SocketFile == "" {
		return fmt.Errorf("Settings uses '%v' alerts transport, but has no socket"+
			" file.", UNIX_SOCKET_TRANSPORT)
	}
	if err := s.validatePersistFile(); err != nil {
		return err
	}
	return nil
}

// Validates a process group config.
func (c *ConfigManager) validate() error {
	if len(c.ProcessGroups) == 0 {
		return fmt.Errorf("A configuration file (*-gonit.yml) must be provided.")
	}
	for _, pg := range c.ProcessGroups {
		if err := pg.validateRequiredFieldsExist(); err != nil {
			return err
		}
		if err := pg.validateLinks(); err != nil {
			return err
		}
	}
	if err := c.Settings.validate(); err != nil {
		return err
	}
	return nil
}

func (p *Process) IsMonitoringModeActive() bool {
	return p.MonitorMode == MONITOR_MODE_ACTIVE
}

func (p *Process) IsMonitoringModePassive() bool {
	return p.MonitorMode == MONITOR_MODE_PASSIVE
}

func (p *Process) IsMonitoringModeManual() bool {
	return p.MonitorMode == MONITOR_MODE_MANUAL
}
