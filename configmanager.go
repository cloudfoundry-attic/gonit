// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"fmt"
	"github.com/xushiwei/goyaml"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// TODO:
// - Global interval for collecting resources.
// - Accept a config option for setting how alerts are sent (unix socket, smtp,
//	    etc.)
// - Throw an error if the interval * MAX_DATA_TO_STORE < a rule's duration e.g.
//       if interval is 1s and MAX_DATA_TO_STORE = 120 and someone wants 3
//       minutes duration in a rule.

type ConfigManager struct {
	ProcessGroups map[string]*ProcessGroup
	Settings      *Settings
}

type Settings struct {
	AlertTransport string
	SocketFile     string
}

type ProcessGroup struct {
	Name      string
	Events    map[string]Event
	Processes map[string]Process
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
	Description string
	Pidfile     string
	Start       string
	Stop        string
	DependsOn   []string
	Actions     map[string][]string
	Gid         string
	Uid         string
	// TODO How do we make it so Monitor is true by default and only false when
	// explicitly set in yaml?
	Monitor bool
}

const (
	CONFIG_FILE_POSTFIX   = "-gonit.yml"
	SETTINGS_FILENAME     = "gonit.yml"
	UNIX_SOCKET_TRANSPORT = "unix_socket"
)

const (
	DEFAULT_ALERT_TRANSPORT = "none"
)

// Gets the PID file for a process and returns the PID for it.
func (p Process) GetPid() (int, error) {
	pidfile, err := ioutil.ReadFile(p.Pidfile)
	if err != nil {
		return -1, err
	}
	pidfileInt, err := strconv.Atoi(string(pidfile))
	if err != nil {
		return -1, err
	}
	return pidfileInt, nil
}

// Given an action string name, returns the events associated with it.
func (pg ProcessGroup) EventsFromAction(actionEvent string) []Event {
	events := []Event{}
	for _, event := range pg.Events {
		if event.Name == actionEvent {
			events = append(events, event)
		}
	}
	return events
}

// Given a process name, returns the Process and whether it exists.
func (pg ProcessGroup) processFromName(name string) (Process, bool) {
	serv, hasKey := pg.Processes[name]
	return serv, hasKey
}

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
func (c *ConfigManager) parseFile(path string) (*ProcessGroup, error) {
	processGroup := &ProcessGroup{}
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := goyaml.Unmarshal(b, processGroup); err != nil {
		return nil, err
	}
	log.Printf("Loaded config file '%+v'\n", path)
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
	log.Printf("Loaded settings file: '%+v'\n", path)
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

	dirNames, err := dir.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, filename := range dirNames {
		if strings.HasSuffix(filename, CONFIG_FILE_POSTFIX) {
			c.ProcessGroups[getGroupName(filename)], _ =
				c.parseFile(filepath.Join(dirPath, filename))
			if err != nil {
				return err
			}
		} else if filename == SETTINGS_FILENAME {
			c.Settings, err = c.parseSettingsFile(filepath.Join(dirPath, filename))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *ConfigManager) applyDefaultSettings() {
	settings := c.Settings
	if settings.AlertTransport == "" {
		settings.AlertTransport = DEFAULT_ALERT_TRANSPORT
	}
}

// Main function to call, parses a path for gonit config file(s).
func (c *ConfigManager) Parse(paths ...string) error {
	c.ProcessGroups = map[string]*ProcessGroup{}
	for _, path := range paths {
		fileInfo, err := os.Stat(path)
		if err != nil {
			log.Printf("Error stating path '%+v'.\n", path)
		}
		if fileInfo.IsDir() {
			if err = c.parseDir(path); err != nil {
				return err
			}
		} else {
			_, filename := filepath.Split(path)
			groupName := getGroupName(filename)
			if filename == SETTINGS_FILENAME {
				if c.Settings, err = c.parseSettingsFile(path); err != nil {
					return err
				}
			} else if strings.HasSuffix(path, CONFIG_FILE_POSTFIX) {
				if c.ProcessGroups[groupName], err = c.parseFile(path); err != nil {
					return err
				}
			}
		}
		c.fillInNames()
	}
	if c.Settings == nil {
		log.Printf("No settings found, using defaults.")
	}
	c.applyDefaultSettings()
	if err := c.validate(); err != nil {
		log.Fatal(err)
	}
	return nil
}

// Validates that certain fields exist in the config file.
func (pg ProcessGroup) validateRequiredFieldsExist() error {
	for name, process := range pg.Processes {
		if process.Name == "" || process.Description == "" ||
			process.Pidfile == "" || process.Start == "" || process.Stop == "" ||
			process.Gid == "" || process.Uid == "" {
			return fmt.Errorf("%v must have name, description, pidfile, start, "+
				"stop, gid, and uid.", name)
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
	return nil
}

// Validates a process group config.
func (c *ConfigManager) validate() error {
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
