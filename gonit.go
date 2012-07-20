// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"fmt"
)

const (
	TYPE_FILESYSTEM = iota
	TYPE_DIRECTORY
	TYPE_FILE
	TYPE_PROCESS
	TYPE_HOST
	TYPE_SYSTEM
	TYPE_FIFO
	TYPE_PROGRAM
)

const (
	MODE_ACTIVE = iota
	MODE_PASSIVE
	MODE_MANUAL
)

const (
	MONITOR_NOT     = 0x0
	MONITOR_YES     = 0x1
	MONITOR_INIT    = 0x2
	MONITOR_WAITING = 0x4
)

const (
	ACTION_IGNORE = iota
	ACTION_ALERT
	ACTION_RESTART
	ACTION_STOP
	ACTION_EXEC
	ACTION_UNMONITOR
	ACTION_START
	ACTION_MONITOR
)

type service struct {
	name          string
	stype         int // TYPE_*
	monitor       int // MONITOR_*
	mode          int // MODE_*
	nstart        int
	visited       bool
	dependVisited bool
	dependsOn     []string
	process       *Daemon // only for TYPE_PROCESS
}

type gonit struct {
	serviceList []*service
}

func (g *gonit) findService(name string) (*service, error) {
	for _, s := range g.serviceList {
		if s.name == name {
			return s, nil
		}
	}

	return nil, fmt.Errorf("service %q not found", name)
}

func (g *gonit) addService(s *service) error {
	if _, err := g.findService(s.name); err == nil {
		return fmt.Errorf("service %q already exists", s.name)
	}
	g.serviceList = append(g.serviceList, s)
	return nil
}
