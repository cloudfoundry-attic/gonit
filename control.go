// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"fmt"
	"github.com/xushiwei/goyaml"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"
)

const (
	MONITOR_NOT     = 0x0
	MONITOR_YES     = 0x1
	MONITOR_INIT    = 0x2
	MONITOR_WAITING = 0x4
)

const (
	ACTION_START = iota
	ACTION_STOP
	ACTION_RESTART
	ACTION_MONITOR
	ACTION_UNMONITOR
)

var PersistPath = os.Getenv("HOME") + "/.gonit.persist.yml"

const (
	processStopped = iota
	processStarted
)

// So we can mock it in tests.
type EventMonitorInterface interface {
	StartMonitoringProcess(process *Process)
	Start(configManager *ConfigManager, control *Control) error
	Stop()
}

type Control struct {
	configManager *ConfigManager
	EventMonitor  EventMonitorInterface
	visits        map[string]*visitor
	states        map[string]*ProcessState
	persistLock   sync.Mutex
}

// flags to avoid invoking actions more than once
// as we traverse the dependency graph
type visitor struct {
	started bool
	stopped bool
}

// XXX TODO should state be attached to Process type?
type ProcessState struct {
	Monitor     int
	MonitorLock sync.Mutex
	Starts      int
}

// XXX TODO needed for tests, a form of this should probably be in ConfigManager
func (c *ConfigManager) AddProcess(groupName string, process *Process) error {
	groups := c.ProcessGroups
	var group *ProcessGroup
	var exists bool
	if group, exists = groups[groupName]; !exists {
		group = &ProcessGroup{
			Name:      groupName,
			Processes: make(map[string]*Process),
		}
		groups[groupName] = group
	}

	if _, exists := group.Processes[process.Name]; exists {
		return fmt.Errorf("process %q already exists", process.Name)
	} else {
		group.Processes[process.Name] = process
	}

	return nil
}

// BUG(lisbakke): If there are two processes named the same thing in different process groups, this could return the wrong process. ConfigManager should enforce unique group/process names.
// XXX TODO should probably be in configmanager.go
// Helper methods to find a Process by name
func (c *ConfigManager) FindProcess(name string) (*Process, error) {
	for _, processGroup := range c.ProcessGroups {
		if process, exists := processGroup.Processes[name]; exists {
			return process, nil
		}
	}

	return nil, fmt.Errorf("process %q not found", name)
}

// TODO should probably be in configmanager.go
// Helper method to find a ProcessGroup by name
func (c *ConfigManager) FindGroup(name string) (*ProcessGroup, error) {
	if group, exists := c.ProcessGroups[name]; exists {
		return group, nil
	}
	return nil, fmt.Errorf("process group %q not found", name)
}

// configManager accessor (exported for tests)
func (c *Control) Config() *ConfigManager {
	if c.configManager == nil {
		c.configManager = &ConfigManager{}
	}

	if c.configManager.ProcessGroups == nil {
		// XXX TODO NewConfigManager() ?
		c.configManager.ProcessGroups = make(map[string]*ProcessGroup)
	}

	return c.configManager
}

// XXX TODO should probably be in configmanager.go
// Visit each Process in the ConfigManager.
// Stop visiting if visit func returns false
func (c *ConfigManager) VisitProcesses(visit func(p *Process) bool) {
	for _, processGroup := range c.ProcessGroups {
		for _, process := range processGroup.Processes {
			if !visit(process) {
				return
			}
		}
	}
}

func (c *Control) visitorOf(process *Process) *visitor {
	if _, exists := c.visits[process.Name]; !exists {
		c.visits[process.Name] = &visitor{}
	}

	return c.visits[process.Name]
}

func (c *Control) State(process *Process) *ProcessState {
	if c.states == nil {
		c.states = make(map[string]*ProcessState)
	}
	procName := process.Name
	if _, exists := c.states[procName]; !exists {
		loadedPersist := false
		// If there was a persisted one, load that instead.
		if persistedState, err := c.LoadPersistState(); err == nil {
			if state, exists := persistedState.ProcessStates[procName]; exists {
				log.Printf("\"%v\" loaded persisted state \"%+v\".", procName, state)
				c.states[procName] = &state
				loadedPersist = true
			}
		}
		if !loadedPersist {
			c.states[procName] = &ProcessState{}
		}
	}
	return c.states[procName]
}

// Registers the event monitor with Control so that it can turn event monitoring
// on/off when processes are started/stopped.
func (c *Control) RegisterEventMonitor(eventMonitor *EventMonitor) {
	c.EventMonitor = eventMonitor
}

// Invoke given action for the given process and its
// dependents and/or dependencies
func (c *Control) DoAction(name string, action int) error {
	c.visits = make(map[string]*visitor)

	process, err := c.Config().FindProcess(name)
	if err != nil {
		log.Print(err)
		return err
	}

	switch action {
	case ACTION_START:
		if process.IsRunning() {
			log.Printf("Process %q already running", name)
			c.monitorSet(process)
			return nil
		}
		c.doDepend(process, ACTION_STOP)
		c.doStart(process)
		c.doDepend(process, ACTION_START)

	case ACTION_STOP:
		c.doDepend(process, ACTION_STOP)
		c.doStop(process)

	case ACTION_RESTART:
		c.doDepend(process, ACTION_STOP)
		if c.doStop(process) {
			c.doStart(process)
			c.doDepend(process, ACTION_START)
		} else {
			c.monitorSet(process)
		}

	case ACTION_MONITOR:
		c.doMonitor(process)

	case ACTION_UNMONITOR:
		c.doDepend(process, ACTION_UNMONITOR)
		c.doUnmonitor(process)

	default:
		err = fmt.Errorf("process %q -- invalid action: %d",
			process.Name, action)
		log.Print(err)
		return err
	}

	return nil
}

// Start the given Process dependencies before starting Process
func (c *Control) doStart(process *Process) {
	visitor := c.visitorOf(process)
	if visitor.started {
		return
	}
	visitor.started = true

	for _, d := range process.DependsOn {
		parent, err := c.Config().FindProcess(d)
		if err != nil {
			panic(err)
		}
		c.doStart(parent)
	}

	if !process.IsRunning() {
		c.State(process).Starts++
		if err := c.PersistState(); err != nil {
			log.Printf("Error persisting state: '%v'.\n", err.Error())
		}
		process.StartProcess()
		process.waitState(processStarted)
	}

	c.monitorSet(process)
}

// Stop the given Process.
// Waits for process to stop or until Process.Timeout is reached.
func (c *Control) doStop(process *Process) bool {
	visitor := c.visitorOf(process)
	var rv = true
	if visitor.stopped {
		return rv
	}
	visitor.stopped = true

	if process.IsRunning() {
		process.StopProcess()
		if process.waitState(processStopped) != processStopped {
			rv = false
		}
	}

	c.monitorUnset(process)

	return rv
}

// Enable monitoring for Process dependencies and given Process.
func (c *Control) doMonitor(process *Process) {
	if c.visitorOf(process).started {
		return
	}

	for _, d := range process.DependsOn {
		parent, err := c.Config().FindProcess(d)
		if err != nil {
			panic(err)
		}
		c.doMonitor(parent)
	}

	c.monitorSet(process)
}

// Disable monitoring for the given Process
func (c *Control) doUnmonitor(process *Process) {
	visitor := c.visitorOf(process)
	if visitor.stopped {
		return
	}

	visitor.stopped = true
	c.monitorUnset(process)
}

// Apply actions to processes that depend on the given Process
func (c *Control) doDepend(process *Process, action int) {
	c.configManager.VisitProcesses(func(child *Process) bool {
		for _, dep := range child.DependsOn {
			if dep == process.Name {
				switch action {
				case ACTION_START:
					c.doStart(child)
				case ACTION_MONITOR:
					c.doMonitor(child)
				}

				c.doDepend(child, action)

				switch action {
				case ACTION_STOP:
					c.doStop(child)
				case ACTION_UNMONITOR:
					c.doUnmonitor(child)
				}
				break
			}
		}
		return true
	})
}

func (c *Control) monitorSet(process *Process) {
	state := c.State(process)
	state.MonitorLock.Lock()
	defer state.MonitorLock.Unlock()
	if state.Monitor == MONITOR_NOT {
		state.Monitor = MONITOR_INIT
		if err := c.PersistState(); err != nil {
			log.Printf("Error persisting state in monitorSet.\n")
		}
		c.EventMonitor.StartMonitoringProcess(process)
		log.Printf("%q monitoring enabled", process.Name)
	}
}

func (c *Control) monitorUnset(process *Process) {
	state := c.State(process)
	state.MonitorLock.Lock()
	defer state.MonitorLock.Unlock()
	if state.Monitor != MONITOR_NOT {
		state.Monitor = MONITOR_NOT
		if err := c.PersistState(); err != nil {
			log.Printf("Error persisting state in monitorUnset.\n")
		}
		log.Printf("%q monitoring disabled", process.Name)
	}
}

func (c *Control) IsMonitoring(process *Process) bool {
	state := c.State(process)
	state.MonitorLock.Lock()
	defer state.MonitorLock.Unlock()
	return state.Monitor == MONITOR_INIT || state.Monitor == MONITOR_YES
}

// Poll process for expected state change
func (p *Process) pollState(timeout time.Duration, expect int) bool {
	isRunning := false
	timeoutTicker := time.NewTicker(timeout)
	pollTicker := time.NewTicker(100 * time.Millisecond)
	defer timeoutTicker.Stop()
	defer pollTicker.Stop()

	// XXX TODO could make use of psnotify + fsnotify here
	for {
		select {
		case <-timeoutTicker.C:
			return isRunning
		case <-pollTicker.C:
			isRunning = p.IsRunning()

			if (expect == processStopped && !isRunning) ||
				(expect == processStarted && isRunning) {
				return isRunning
			}
		}
	}

	panic("not reached")
}

// Wait for a Process to change state.
func (p *Process) waitState(expect int) int {
	timeout := 30 * time.Second // XXX TODO process.Timeout
	isRunning := p.pollState(timeout, expect)

	// XXX TODO emit events when process state changes
	if isRunning {
		if expect == processStarted {
			log.Printf("process %q started", p.Name)
		} else {
			log.Printf("process %q failed to stop", p.Name)
		}
		return processStarted
	} else {
		if expect == processStarted {
			log.Printf("process %q failed to start", p.Name)
		} else {
			log.Printf("process %q stopped", p.Name)
		}
		return processStopped
	}

	panic("not reached")
}

type PersistedState struct {
	ProcessStates map[string]ProcessState
}

func (c *Control) PersistState() error {
	c.persistLock.Lock()
	defer c.persistLock.Unlock()
	persistedState, err := c.LoadPersistState()
	if err != nil {
		return err
	}
	for _, processGroup := range c.configManager.ProcessGroups {
		for name, process := range processGroup.Processes {
			persistedState.ProcessStates[name] = *c.State(process)
		}
	}
	yaml, err := goyaml.Marshal(persistedState)
	if err != nil {
		return err
	}
	if err = ioutil.WriteFile(PersistPath, []byte(yaml), 0644); err != nil {
		return err
	}
	log.Printf("Persisted config to '%v'.", PersistPath)
	return nil
}

func (c *Control) LoadPersistState() (*PersistedState, error) {
	persistedState := &PersistedState{ProcessStates: map[string]ProcessState{}}
	_, err := os.Stat(PersistPath)
	if err != nil {
		// If we don't have a persisted file, don't worry about it.
		return persistedState, nil
	}
	persistData, err := ioutil.ReadFile(PersistPath)
	if err != nil {
		return nil, err
	}
	if err := goyaml.Unmarshal(persistData, persistedState); err != nil {
		return nil, err
	}
	return persistedState, nil
}
