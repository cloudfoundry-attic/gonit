// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"log"
	"time"
)

const (
	processStopped = iota
	processStarted
)

func (g *gonit) controlService(name string, action int) bool {
	s, err := g.findService(name)
	if err != nil {
		log.Print(err)
		return false
	}

	switch action {
	case ACTION_START:
		if s.stype == TYPE_PROCESS {
			if s.process.IsRunning() {
				log.Printf("Process already running -- process %s", name)
				s.monitorSet()
				return true
			}
			g.doDepend(s, ACTION_STOP, false)
			g.doStart(s, false)
			g.doDepend(s, ACTION_START, false)
		}
	case ACTION_STOP:
		g.doDepend(s, ACTION_STOP, true)
		g.doStop(s, true)
	}

	return true
}

func (g *gonit) doStart(s *service, flag bool) {
	if s.visited {
		return
	}
	s.visited = true

	for _, d := range s.dependsOn {
		parent, err := g.findService(d)
		if err != nil {
			panic(err)
		}
		g.doStart(parent, flag)
	}

	if s.stype == TYPE_PROCESS {
		s.nstart++
		s.process.StartProcess()
		g.waitProcess(s, processStarted)
	}

	s.monitorSet()
}

func (g *gonit) doStop(s *service, flag bool) bool {
	var rv = true
	if s.dependVisited {
		return rv
	}
	s.dependVisited = true

	if s.stype == TYPE_PROCESS && s.process.IsRunning() {
		s.process.StopProcess()
		if g.waitProcess(s, processStopped) != processStopped {
			rv = false
		}
	}

	if flag {
		s.monitorUnset()
	} else {
		// XXX resetInfo
	}

	return rv
}

func (g *gonit) doMonitor(s *service, flag bool) {
	if s.visited {
		return
	}

	for _, d := range s.dependsOn {
		parent, err := g.findService(d)
		if err != nil {
			panic(err)
		}
		g.doMonitor(parent, flag)
	}

	s.monitorSet()
}

func (g *gonit) doUnmonitor(s *service, flag bool) {
	if s.dependVisited {
		return
	}

	s.dependVisited = true
	s.monitorUnset()
}

func (g *gonit) doDepend(s *service, action int, flag bool) {
	for _, child := range g.serviceList {
		for _, d := range child.dependsOn {
			if d == s.name {
				switch action {
				case ACTION_START:
					g.doStart(child, flag)
				case ACTION_MONITOR:
					g.doMonitor(child, flag)
				}

				g.doDepend(child, action, flag)

				switch action {
				case ACTION_STOP:
					g.doStop(child, flag)
				case ACTION_UNMONITOR:
					g.doUnmonitor(child, flag)
				}
				break
			}
		}
	}
}

func (g *gonit) waitProcess(s *service, expect int) int {
	time.Sleep(100 * time.Millisecond) // XXX

	isRunning := s.process.IsRunning()

	if isRunning {
		if expect == processStarted {
			// started
		} else {
			// failed to stop
		}
		return processStarted
	} else {
		if expect == processStarted {
			// failed to start
		} else {
			// stopped
		}
		return processStopped
	}

	panic("not reached")
}

func (g *gonit) resetDepend() {
	for _, s := range g.serviceList {
		s.visited = false
		s.dependVisited = false
	}
}

func (s *service) monitorSet() {
	if s.monitor == MONITOR_NOT {
		s.monitor = MONITOR_INIT
		log.Printf("%q monitoring enabled", s.name) // XXX log.Debug
	}
}

func (s *service) monitorUnset() {
	if s.monitor != MONITOR_NOT {
		s.monitor = MONITOR_NOT
		log.Printf("%q monitoring disabled", s.name) // XXX log.Debug
	}
	// XXX more
}
