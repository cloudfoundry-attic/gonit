// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"fmt"
	"github.com/bmizerany/assert"
	"testing"
)

func NewTestService(name string, flags []string) *service {
	daemon := NewTestDaemon(name, flags, false)
	service := &service{
		stype:   TYPE_PROCESS,
		name:    daemon.Name,
		process: daemon,
	}
	return service
}

func TestStartStop(t *testing.T) {
	g := &gonit{}

	name := "simple"
	service := NewTestService(name, nil)
	defer cleanup(service.process)

	err := g.addService(service)
	assert.Equal(t, nil, err)

	assert.Equal(t, MONITOR_NOT, service.monitor)

	rv := g.controlService(name, ACTION_START)
	assert.Equal(t, true, rv)

	assert.Equal(t, MONITOR_INIT, service.monitor)

	assert.Equal(t, true, service.process.IsRunning())

	rv = g.controlService(name, ACTION_STOP)
	assert.Equal(t, true, rv)

	assert.Equal(t, MONITOR_NOT, service.monitor)
}

func TestDepends(t *testing.T) {
	g := &gonit{}

	name := "depsimple"
	service := NewTestService(name, nil)
	defer cleanup(service.process)

	nservices := 4
	var oservices []string

	for i := 0; i < nservices; i++ {
		dname := fmt.Sprintf("%s_dep_%d", name, i)
		dservice := NewTestService(dname, nil)
		defer cleanup(dservice.process)

		err := g.addService(dservice)
		assert.Equal(t, nil, err)
		if i%2 == 0 {
			service.dependsOn = append(service.dependsOn, dname)
		} else {
			oservices = append(oservices, dname)
		}
	}

	err := g.addService(service)
	assert.Equal(t, nil, err)

	// start main service
	rv := g.controlService(name, ACTION_START)
	assert.Equal(t, true, rv)

	assert.Equal(t, true, service.process.IsRunning())

	// stop main service
	rv = g.controlService(name, ACTION_STOP)
	assert.Equal(t, true, rv)
	assert.Equal(t, false, service.process.IsRunning())

	// check start count for main service and deps
	assert.Equal(t, 1, service.nstart)
	for _, dname := range service.dependsOn {
		dservice, _ := g.findService(dname)
		assert.Equal(t, 1, dservice.nstart)
	}

	for _, oname := range oservices {
		oservice, _ := g.findService(oname)
		assert.Equal(t, 0, oservice.nstart)
	}

	g.resetDepend()

	// test start/stop of dependant

	// start main sevice
	rv = g.controlService(name, ACTION_START)
	assert.Equal(t, true, rv)
	assert.Equal(t, true, service.process.IsRunning())

	// stop a dependency
	rv = g.controlService(service.dependsOn[0], ACTION_STOP)
	assert.Equal(t, true, rv)

	assert.Equal(t, false, service.process.IsRunning())

	// start a dependency
	rv = g.controlService(service.dependsOn[0], ACTION_START)
	assert.Equal(t, true, rv)

	// stop main service
	rv = g.controlService(name, ACTION_STOP)
	assert.Equal(t, true, rv)

	assert.Equal(t, 2, service.nstart)
}
