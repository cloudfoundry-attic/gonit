// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"fmt"
	"github.com/xushiwei/goyaml"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
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
	Daemon
	Description string
	DependsOn   []string
	Actions     map[string][]string
	// TODO How do we make it so Monitor is true by default and only false when
	// explicitly set in yaml?
	Monitor bool
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
func (c *ConfigManager) parseFile(filename string) (*ProcessGroup, error) {
	processGroup := &ProcessGroup{}
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	if err := goyaml.Unmarshal(b, processGroup); err != nil {
		return nil, err
	}
	log.Printf("loaded '%+v'\n", filename)
	return processGroup, nil
}

// Given a file path, gets the filename and removes -gonit.yml.
func getGroupName(gonitFilePath string) string {
	_, filename := filepath.Split(gonitFilePath)
	// -10 because of "-gonit.yml"
	return filename[:len(filename)-10]
}

// Parses a directory for gonit files.
func (c *ConfigManager) parseDir(dirPath string) (map[string]*ProcessGroup,
	error) {
	processGroups := c.ProcessGroups
	dir, err := os.Open(dirPath)
	if err != nil {
		return nil, err
	}

	dirNames, err := dir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	for _, filename := range dirNames {
		if strings.HasSuffix(filename, "-gonit.yml") {
			processGroups[getGroupName(filename)], _ =
				c.parseFile(filepath.Join(dirPath, filename))
		}
	}
	return processGroups, nil
}

// Main function to call, parses a path for gonit config file(s).
func (c *ConfigManager) Parse(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		log.Printf("Error stating path '%+v'.\n", path)
	}
	c.ProcessGroups = map[string]*ProcessGroup{}
	if fileInfo.IsDir() {
		if c.ProcessGroups, err = c.parseDir(path); err != nil {
			return err
		}
	} else {
		groupName := getGroupName(path)
		if c.ProcessGroups[groupName], err = c.parseFile(path); err != nil {
			return err
		}
	}
	c.fillInNames()

	for _, processGroup := range c.ProcessGroups {
		if err := processGroup.validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validates that certain fields exist in the config file.
func (pg ProcessGroup) validateRequiredFieldsExist() error {
	for name, process := range pg.Processes {
		if process.Name == "" || process.Description == "" ||
			process.Pidfile == "" || process.Start == "" {
			return fmt.Errorf("%v must have name, description, pidfile and start.", name)
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
func (pg ProcessGroup) validateLinks() error {
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

// Validates a process group config.
func (pg ProcessGroup) validate() error {
	if err := pg.validateRequiredFieldsExist(); err != nil {
		return err
	}
	if err := pg.validateLinks(); err != nil {
		return err
	}
	return nil
}
