// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"errors"
	"github.com/cloudfoundry/gosigar"
	"log"
)

// until stubs are implemented
var notimpl = errors.New("Method not implemented")

type API struct {
	Control *Control
}

type ProcessSummary struct {
	Name         string
	Running      bool
	ControlState processState
}

type ProcessStatus struct {
	Summary ProcessSummary
	Pid     int
	State   sigar.ProcState
	Time    sigar.ProcTime
	Mem     sigar.ProcMem
}

type SystemStatus struct {
	// XXX load, cpu, mem, swap, etc
}

type ProcessGroupStatus struct {
	Name  string
	Group []ProcessStatus
}

type Summary struct {
	Processes []ProcessSummary
}

type About struct {
	Version    string
	Id         string
	Incaration uint64
}

type ActionResult struct {
	Total  int
	Errors int
}

// wrap errors returned by API methods so client can
// disambiguate between API errors and rpc errors
type ActionError struct {
	Err error
}

func (e *ActionError) Error() string {
	return "ActionError: " + e.Err.Error()
}

func NewAPI(config *ConfigManager) *API {
	return &API{
		Control: &Control{configManager: config},
	}
}

// *Process methods apply to a single service

func (c *Control) callAction(name string, r *ActionResult, action int) error {
	err := c.DoAction(name, action)

	r.Total++
	if err != nil {
		r.Errors++
		err = &ActionError{err}
	}

	return err
}

func (a *API) startNewAndOldProcesses(runningProcesses map[string]bool,
	oldProcesses map[string]bool, r *ActionResult) error {
	for _, processGroup := range a.Control.Config().ProcessGroups {
		for name, _ := range processGroup.Processes {
			// If it's an old running process still in the process map, restart it.
			if _, hasKey := runningProcesses[name]; hasKey {
				if err := a.StartProcess(name, r); err != nil {
					return err
				}
			} else if _, hasKey := oldProcesses[name]; !hasKey {
				// Start it if it is a new process.
				if err := a.StartProcess(name, r); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (a *API) LoadConfig(name string, r *ActionResult) error {
	log.Printf("Starting config reload.")
	control := a.Control
	control.EventMonitor.Stop()
	runningProcesses := map[string]bool{}
	oldProcesses := map[string]bool{}
	for _, processGroup := range control.Config().ProcessGroups {
		for name, process := range processGroup.Processes {
			oldProcesses[name] = true
			if process.IsRunning() {
				runningProcesses[name] = true
				if err := a.StopProcess(name, r); err != nil {
					return err
				}
			}
		}
	}
	if err := control.allAction(r, ACTION_STOP); err != nil {
		return err
	}
	if err := control.configManager.LoadConfig(name); err != nil {
		return err
	}
	if err := a.startNewAndOldProcesses(runningProcesses,
		oldProcesses, r); err != nil {
		return err
	}
	if err := control.EventMonitor.Start(control.configManager,
		control); err != nil {
		return err
	}
	log.Printf("Finished config reload.")
	return nil
}

func (a *API) StartProcess(name string, r *ActionResult) error {
	return a.Control.callAction(name, r, ACTION_START)
}

func (a *API) StopProcess(name string, r *ActionResult) error {
	return a.Control.callAction(name, r, ACTION_STOP)
}

func (a *API) RestartProcess(name string, r *ActionResult) error {
	return a.Control.callAction(name, r, ACTION_RESTART)
}

func (a *API) MonitorProcess(name string, r *ActionResult) error {
	return a.Control.callAction(name, r, ACTION_MONITOR)
}

func (a *API) UnmonitorProcess(name string, r *ActionResult) error {
	return a.Control.callAction(name, r, ACTION_UNMONITOR)
}

func (c *Control) processSummary(process *Process, summary *ProcessSummary) {
	summary.Name = process.Name
	summary.Running = process.IsRunning()
	summary.ControlState = *c.State(process)
}

func (c *Control) processStatus(process *Process, status *ProcessStatus) error {
	c.processSummary(process, &status.Summary)

	if !status.Summary.Running {
		return nil
	}

	pid, err := process.Pid()
	if err != nil {
		return err
	}
	status.Pid = pid

	status.State.Get(pid)
	status.Time.Get(pid)
	status.Mem.Get(pid)

	return nil
}

func (a *API) StatusProcess(name string, r *ProcessStatus) error {
	process, err := a.Control.Config().FindProcess(name)

	if err != nil {
		return err
	}

	return a.Control.processStatus(process, r)
}

// *Group methods apply to a service group

func (c *Control) groupAction(name string, r *ActionResult, action int) error {
	group, err := c.Config().FindGroup(name)

	if err != nil {
		return &ActionError{err}
	}

	for name := range group.Processes {
		c.callAction(name, r, action)
	}

	return nil
}

func (a *API) StartGroup(name string, r *ActionResult) error {
	return a.Control.groupAction(name, r, ACTION_START)
}

func (a *API) StopGroup(name string, r *ActionResult) error {
	return a.Control.groupAction(name, r, ACTION_STOP)
}

func (a *API) RestartGroup(name string, r *ActionResult) error {
	return a.Control.groupAction(name, r, ACTION_RESTART)
}

func (a *API) MonitorGroup(name string, r *ActionResult) error {
	return a.Control.groupAction(name, r, ACTION_MONITOR)
}

func (a *API) UnmonitorGroup(name string, r *ActionResult) error {
	return a.Control.groupAction(name, r, ACTION_UNMONITOR)
}

func (c *Control) groupStatus(group *ProcessGroup,
	groupStatus *ProcessGroupStatus) error {

	for _, process := range group.Processes {
		status := ProcessStatus{}
		c.processStatus(process, &status)
		groupStatus.Group = append(groupStatus.Group, status)
	}

	return nil
}

func (a *API) StatusGroup(name string, r *ProcessGroupStatus) error {
	group, err := a.Control.Config().FindGroup(name)

	if err != nil {
		return err
	}

	r.Name = name
	a.Control.groupStatus(group, r)

	return nil
}

// *All methods apply to all services

func (c *Control) allAction(r *ActionResult, action int) error {
	for _, processGroup := range c.Config().ProcessGroups {
		for name, _ := range processGroup.Processes {
			c.callAction(name, r, action)
		}
	}
	return nil
}

func (a *API) StartAll(unused interface{}, r *ActionResult) error {
	return a.Control.allAction(r, ACTION_START)
}

func (a *API) StopAll(unused interface{}, r *ActionResult) error {
	return a.Control.allAction(r, ACTION_STOP)
}

func (a *API) RestartAll(unused interface{}, r *ActionResult) error {
	return a.Control.allAction(r, ACTION_RESTART)
}

func (a *API) MonitorAll(unused interface{}, r *ActionResult) error {
	return a.Control.allAction(r, ACTION_MONITOR)
}

func (a *API) UnmonitorAll(unused interface{}, r *ActionResult) error {
	return a.Control.allAction(r, ACTION_UNMONITOR)
}

func (a *API) StatusAll(name string, r *ProcessGroupStatus) error {
	r.Name = name

	for _, processGroup := range a.Control.Config().ProcessGroups {
		a.Control.groupStatus(processGroup, r)
	}

	return nil
}

func (a *API) Summary(unused interface{}, s *Summary) error {
	for _, group := range a.Control.Config().ProcessGroups {
		for _, process := range group.Processes {
			summary := ProcessSummary{}
			a.Control.processSummary(process, &summary)
			s.Processes = append(s.Processes, summary)
		}
	}

	return nil
}

// server info
func (a *API) About(unused interface{}, about *About) error {
	about.Version = VERSION
	return nil
}

// reload server configuration
func (a *API) Reload(unused interface{}, r *ActionResult) error {
	return notimpl
}

// quit server daemon
func (a *API) Quit(unused interface{}, r *ActionResult) error {
	return notimpl
}
