// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"fmt"
	"io/ioutil"
	"launchpad.net/goyaml"
	"os"
	"path/filepath"
	"strconv"
	"strings"

/*		"flag"*/
)

/*var load_path = flag.String("load_path", "", "The path to *-gonit.yml file(s).")*/

type ConfigManager struct {
	ProcessGroups map[string]ProcessGroup
}

/* TODO:
- Clean up inline todos
- Fix the overall organization of things.  E.g. all of the functions on ProcessGroup that return ProcessGroup.
- Add more utility functions.
- Turn this into a package for others to use.
- Clean up error handling
- Add better validation
- plenty of other things to do
*/
type ProcessGroup struct {
	Name      string
	Events    map[string]Event
	Processes map[string]Process
}

type Event struct {
	/* Required. */
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
	/* Required */
	Name        string
	Description string
	Pidfile     string
	Start       string
	Stop        string
	/* Not required */
	DependsOn []string
	Actions   map[string][]string
	Gid       string
	Uid       string
}

/*todo change the prints to debug messages*/
func (p Process) GetPid() (int, error) {
	pidfile, err := ioutil.ReadFile(p.Pidfile)
	if err != nil {
		return -1, err
	} // TODO: fix.
	pidfileInt, err := strconv.Atoi(string(pidfile))
	if err != nil {
		return -1, err
	}
	return pidfileInt, nil
}

func (pg ProcessGroup) EventsFromAction(actionEvents []string) []Event {
	events := []Event{}
	for _, eventName := range actionEvents {
		for _, event := range pg.Events {
			if event.Name == eventName {
				events = append(events, event)
			}
		}
	}
	return events
}

func (pg ProcessGroup) processFromDepends(dependsOnName string) (Process, bool) {
	serv, hasKey := pg.Processes[dependsOnName]
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

func (c ConfigManager) parseFilename(filename string) ProcessGroup {
	processGroup := ProcessGroup{}
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	} // TODO: fix.
	err2 := goyaml.Unmarshal(b, &processGroup)
	if err2 != nil {
		fmt.Println("error:", err2) // TODO: fix.
	}
	fmt.Printf("loaded '%+v'\n", filename)
	return processGroup
}

func getGroupName(gonitFilePath string) string {
	_, filename := filepath.Split(gonitFilePath)
	// -10 because of "-gonit.yml"
	return filename[:len(filename)-10]
}

func (c ConfigManager) parseDir(dirPath string) map[string]ProcessGroup {
	processGroups := c.ProcessGroups
	dir, dirErr := os.Open(dirPath)
	if dirErr != nil {
		fmt.Println("Error opening directory '%+v'.", dirPath)
	}

	dirNames, readdirErr := dir.Readdirnames(-1)
	if readdirErr != nil {
		fmt.Println("Error opening directory '%+v'.", dirPath)
	}
	for _, filename := range dirNames {
		if strings.HasSuffix(filename, "-gonit.yml") {
			processGroups[getGroupName(filename)] = c.parseFilename(filepath.Join(dirPath, filename))
		}
	}
	return processGroups
}

func (c *ConfigManager) Parse(path string) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		fmt.Println("Error stating path '%+v'.", path)
	}
	c.ProcessGroups = map[string]ProcessGroup{}
	if fileInfo.IsDir() {
		c.ProcessGroups = c.parseDir(path)
	} else {
		c.ProcessGroups[getGroupName(path)] = c.parseFilename(path)
	}
	c.fillInNames()

	for _, processGroup := range c.ProcessGroups {
		processGroup.validate()
	}
}

func (processGroup ProcessGroup) validateRequiredFieldsExist() []string {
	var errors []string
	for name, process := range processGroup.Processes {
		if process.Name == "" || process.Description == "" ||
			process.Pidfile == "" || process.Start == "" || process.Stop == "" ||
			process.Gid == "" || process.Uid == "" {
			errors = append(errors, name+" must have name, description, pidfile, start, stop, gid, and uid.")
		}
	}
	for name, event := range processGroup.Events {
		if event.Name == "" || event.Description == "" || event.Rule == "" {
			errors = append(errors, name+" must have name, description, rule, and actions.")
		}
	}
	return errors
}

func (processGroup ProcessGroup) validateLinks() []string {
	var errors []string
	for _, process := range processGroup.Processes {
		for _, dependsOnName := range process.DependsOn {
			if _, hasKey := processGroup.processFromDepends(dependsOnName); hasKey == false {
				error := "Process " + process.Name + " has an unknown dependson '" + dependsOnName + "'."
				errors = append(errors, error)
			}
		}
	}

	return errors
}

func concatStrArr(arr1 []string, arr2 []string) []string {
	newslice := make([]string, len(arr1)+len(arr2))
	copy(newslice, arr1)
	copy(newslice[len(arr1):], arr2)
	return newslice
}

func (processGroup ProcessGroup) validate() {
	/* todo other things to verify -- */
	var errors []string
	errors = concatStrArr(errors, processGroup.validateRequiredFieldsExist())
	errors = concatStrArr(errors, processGroup.validateLinks())
	if len(errors) > 0 {
		fmt.Printf("Errors: %+v", errors)
	}
}

/*
func main() {
	flag.Parse()
	var processGroup ProcessGroup
	if (*load_path != "") {
		processGroup = processGroup.parse(*load_path)

		fmt.Printf("%+v", processGroup)
	} else {
		fmt.Printf("Must specify load_path.")
	}
}*/
