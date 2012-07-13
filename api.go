// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"errors"
)

// until stubs are implemented
var notimpl = errors.New("Method not implemented")

type API struct {
	// XXX will have context for services, groups, etc.
}

// XXX these types will live elsewhere,
// just enough to test stubs for now
type ServiceStatus struct {
	Name   string
	Type   int
	Mode   int
	Status int
	// depending on Type; one of {Process,File,System,Etc}Status
	Data interface{}
}

type SystemStatus struct {
	// XXX load, cpu, mem, swap, etc
}

type ProcessStatus struct {
	// XXX cpu, mem, etc
}

type ServiceGroupStatus struct {
	Name     string
	Services []ServiceStatus
}

type About struct {
	Version    string
	Id         string
	Incaration uint64
}

type ActionResult struct {
	// XXX
}

// *Service methods apply to a single service

func (a *API) StartService(name string, r *ActionResult) error {
	return notimpl
}

func (a *API) StopService(name string, r *ActionResult) error {
	return notimpl
}

func (a *API) RestartService(name string, r *ActionResult) error {
	return notimpl
}

func (a *API) MonitorService(name string, r *ActionResult) error {
	return notimpl
}

func (a *API) UnmonitorService(name string, r *ActionResult) error {
	return notimpl
}

func (a *API) StatusService(name string, r *ServiceStatus) error {
	return notimpl
}

// *Group methods apply to a service group

func (a *API) StartGroup(name string, r *ActionResult) error {
	return notimpl
}

func (a *API) StopGroup(name string, r *ActionResult) error {
	return notimpl
}

func (a *API) RestartGroup(name string, r *ActionResult) error {
	return notimpl
}

func (a *API) MonitorGroup(name string, r *ActionResult) error {
	return notimpl
}

func (a *API) UnmonitorGroup(name string, r *ActionResult) error {
	return notimpl
}

func (a *API) StatusGroup(name string, r *ServiceGroupStatus) error {
	return notimpl
}

// *All methods apply to all services

func (a *API) StartAll(unused interface{}, r *ActionResult) error {
	return notimpl
}

func (a *API) StopAll(unused interface{}, r *ActionResult) error {
	return notimpl
}

func (a *API) RestartAll(unused interface{}, r *ActionResult) error {
	return notimpl
}

func (a *API) MonitorAll(unused interface{}, r *ActionResult) error {
	return notimpl
}

func (a *API) UnmonitorAll(unused interface{}, r *ActionResult) error {
	return notimpl
}

func (a *API) StatusAll(name string, r *ServiceGroupStatus) error {
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
