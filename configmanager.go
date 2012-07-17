// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"fmt"
	"io/ioutil"
	"github.com/xushiwei/goyaml"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ConfigManager struct {
	ProcessGroups map[string]ProcessGroup
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
}

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
func (c ConfigManager) parseFile(filename string) (ProcessGroup, error) {
	processGroup := ProcessGroup{}
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return ProcessGroup{}, err
	}
	if err := goyaml.Unmarshal(b, &processGroup); err != nil {
		return ProcessGroup{}, err
	}
	fmt.Printf("loaded '%+v'\n", filename)
	return processGroup, nil
}

// Given a file path, gets the filename and removes -gonit.yml.
func getGroupName(gonitFilePath string) string {
	_, filename := filepath.Split(gonitFilePath)
	// -10 because of "-gonit.yml"
	return filename[:len(filename)-10]
}

// Parses a directory for gonit files.
func (c ConfigManager) parseDir(dirPath string) (map[string]ProcessGroup, error) {
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
		fmt.Println("Error stating path '%+v'.", path)
	}
	c.ProcessGroups = map[string]ProcessGroup{}
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
