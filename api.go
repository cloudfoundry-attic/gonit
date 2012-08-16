// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"errors"
	"fmt"
)

// until stubs are implemented
var notimpl = errors.New("Method not implemented")

type API struct {
	control *Control
}

type ProcessStatus struct {
	Name   string
	Type   int
	Mode   int
	Status int
	// XXX cpu, mem, etc
}

type SystemStatus struct {
	// XXX load, cpu, mem, swap, etc
}

type ProcessGroupStatus struct {
	Name     string
	Processs []ProcessStatus
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
		control: &Control{configManager: config},
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

func (a *API) StartProcess(name string, r *ActionResult) error {
	return a.control.callAction(name, r, ACTION_START)
}

func (a *API) StopProcess(name string, r *ActionResult) error {
	return a.control.callAction(name, r, ACTION_STOP)
}

func (a *API) RestartProcess(name string, r *ActionResult) error {
	return a.control.callAction(name, r, ACTION_RESTART)
}

func (a *API) MonitorProcess(name string, r *ActionResult) error {
	return a.control.callAction(name, r, ACTION_MONITOR)
}

func (a *API) UnmonitorProcess(name string, r *ActionResult) error {
	return a.control.callAction(name, r, ACTION_UNMONITOR)
}

func (a *API) StatusProcess(name string, r *ProcessStatus) error {
	return notimpl
}

// *Group methods apply to a service group

func (c *Control) groupAction(name string, r *ActionResult, action int) error {
	group, exists := c.Config().ProcessGroups[name]

	if exists {
		for name := range group.Processes {
			c.callAction(name, r, action)
		}
		return nil
	}

	err := fmt.Errorf("process group %q does not exist", name)
	return &ActionError{err}
}

func (a *API) StartGroup(name string, r *ActionResult) error {
	return a.control.groupAction(name, r, ACTION_START)
}

func (a *API) StopGroup(name string, r *ActionResult) error {
	return a.control.groupAction(name, r, ACTION_STOP)
}

func (a *API) RestartGroup(name string, r *ActionResult) error {
	return a.control.groupAction(name, r, ACTION_RESTART)
}

func (a *API) MonitorGroup(name string, r *ActionResult) error {
	return a.control.groupAction(name, r, ACTION_MONITOR)
}

func (a *API) UnmonitorGroup(name string, r *ActionResult) error {
	return a.control.groupAction(name, r, ACTION_UNMONITOR)
}

func (a *API) StatusGroup(name string, r *ProcessGroupStatus) error {
	return notimpl
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
	return a.control.allAction(r, ACTION_START)
}

func (a *API) StopAll(unused interface{}, r *ActionResult) error {
	return a.control.allAction(r, ACTION_STOP)
}

func (a *API) RestartAll(unused interface{}, r *ActionResult) error {
	return a.control.allAction(r, ACTION_RESTART)
}

func (a *API) MonitorAll(unused interface{}, r *ActionResult) error {
	return a.control.allAction(r, ACTION_MONITOR)
}

func (a *API) UnmonitorAll(unused interface{}, r *ActionResult) error {
	return a.control.allAction(r, ACTION_UNMONITOR)
}

func (a *API) StatusAll(name string, r *ProcessGroupStatus) error {
	return notimpl
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
