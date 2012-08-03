// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"fmt"
	"log"
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

const (
	processStopped = iota
	processStarted
)

type Control struct {
	configManager *ConfigManager
	visits        map[string]*visitor
	states        map[string]*processState
}

// flags to avoid invoking actions more than once
// as we traverse the dependency graph
type visitor struct {
	started bool
	stopped bool
}

// XXX TODO should state be attached to Process type?
type processState struct {
	Monitor int
	Starts  int
}

// XXX TODO needed for tests, a form of this should probably be in ConfigManager
func (c *ConfigManager) AddProcess(groupName string, process *Process) error {
	groups := c.ProcessGroups
	var group *ProcessGroup
	var exists bool
	if group, exists = groups[groupName]; !exists {
		group = &ProcessGroup{
			Name:      groupName,
			Processes: make(map[string]Process),
		}
		groups[groupName] = group
	}

	if _, exists := group.Processes[process.Name]; exists {
		return fmt.Errorf("process %q already exists", process.Name)
	} else {
		group.Processes[process.Name] = *process
	}

	return nil
}

// XXX TODO should probably be in configmanager.go
// Helper methods to find a Process by name
func (c *ConfigManager) FindProcess(name string) (*Process, error) {
	for _, processGroup := range c.ProcessGroups {
		if process, exists := processGroup.Processes[name]; exists {
			return &process, nil
		}
	}

	return nil, fmt.Errorf("process %q not found", name)
}

// configManager accessor (exported for tests)
func (c *Control) Config() *ConfigManager {
	if c.configManager == nil {
		// XXX TODO NewConfigManager() ?
		c.configManager = &ConfigManager{
			ProcessGroups: make(map[string]*ProcessGroup),
		}
	}

	return c.configManager
}

// XXX TODO should probably be in configmanager.go
// Visit each Process in the ConfigManager.
// Stop visiting if visit func returns false
func (c *ConfigManager) VisitProcesses(visit func(p *Process) bool) {
	for _, processGroup := range c.ProcessGroups {
		for _, process := range processGroup.Processes {
			if !visit(&process) {
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

func (c *Control) State(process *Process) *processState {
	if c.states == nil {
		c.states = make(map[string]*processState)
	}

	if _, exists := c.states[process.Name]; !exists {
		c.states[process.Name] = &processState{}
	}

	return c.states[process.Name]
}

// Invoke given action for the given process and its
// dependents and/or dependencies
func (c *Control) DoAction(name string, action int) bool {
	c.visits = make(map[string]*visitor)

	process, err := c.Config().FindProcess(name)
	if err != nil {
		log.Print(err)
		return false
	}

	switch action {
	case ACTION_START:
		if process.IsRunning() {
			log.Printf("Process %q already running", name)
			c.monitorSet(process)
			return true
		}
		c.doDepend(process, ACTION_STOP, false)
		c.doStart(process)
		c.doDepend(process, ACTION_START, false)

	case ACTION_STOP:
		c.doDepend(process, ACTION_STOP, true)
		c.doStop(process, true)

	case ACTION_RESTART:
		c.doDepend(process, ACTION_STOP, false)
		if c.doStop(process, false) {
			c.doStart(process)
			c.doDepend(process, ACTION_START, false)
		} else {
			c.monitorSet(process)
		}

	case ACTION_MONITOR:
		c.doMonitor(process)

	case ACTION_UNMONITOR:
		c.doDepend(process, ACTION_UNMONITOR, false)
		c.doUnmonitor(process)

	default:
		log.Printf("process %q -- invalid action: %d",
			process.Name, action)
		return false
	}

	return true
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
		process.StartProcess()
		process.waitState(processStarted)
	}

	c.monitorSet(process)
}

// Stop the given Process.
// Monitoring is disabled when unmonitor flag is true.
// Waits for process to stop or until Process.Timeout is reached.
func (c *Control) doStop(process *Process, unmonitor bool) bool {
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

	if unmonitor {
		c.monitorUnset(process)
	}

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
func (c *Control) doDepend(process *Process, action int, unmonitor bool) {
	c.configManager.VisitProcesses(func(child *Process) bool {
		for _, dep := range child.DependsOn {
			if dep == process.Name {
				switch action {
				case ACTION_START:
					c.doStart(child)
				case ACTION_MONITOR:
					c.doMonitor(child)
				}

				c.doDepend(child, action, unmonitor)

				switch action {
				case ACTION_STOP:
					c.doStop(child, unmonitor)
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

	if state.Monitor == MONITOR_NOT {
		state.Monitor = MONITOR_INIT
		log.Printf("%q monitoring enabled", process.Name)
	}
}

func (c *Control) monitorUnset(process *Process) {
	state := c.State(process)

	if state.Monitor != MONITOR_NOT {
		state.Monitor = MONITOR_NOT
		log.Printf("%q monitoring disabled", process.Name)
	}
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
