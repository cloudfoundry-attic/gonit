// Copyright (c) 2012 VMware, Inc.

package gonit_test

import (
	"fmt"
	"github.com/bmizerany/assert"
	. "github.com/cloudfoundry/gonit"
	"github.com/cloudfoundry/gonit/test/helper"
	"net/rpc"
	"testing"
)

var mockAPI = &MockAPI{api: &API{}}

func init() {
	rpc.Register(mockAPI)
}

type MockAPI struct {
	api *API
}

func (*MockAPI) StopProcess(name string, r *ActionResult) error {
	if name == "foo" {
		return nil
	}
	return fmt.Errorf("Unknown service %s", name)
}

func (*MockAPI) StartGroup(name string, r *ActionResult) error {
	if name == "vcap" {
		return nil
	}
	return fmt.Errorf("Unknown service group %s", name)
}

func (*MockAPI) UnmonitorAll(unused interface{}, r *ActionResult) error {
	return nil
}

func (m *MockAPI) About(args interface{}, about *About) error {
	return m.api.About(args, about)
}

// run tests via RPC or local reflection
func runTests(t *testing.T, client CliClient) {
	method, name := RpcArgs("stop", "foo", false)
	_, err := client.Call(method, name)
	assert.Equal(t, nil, err)

	method, name = RpcArgs("stop", "bar", false)
	_, err = client.Call(method, name)
	assert.NotEqual(t, nil, err)

	method, name = RpcArgs("unmonitor", "all", false)
	_, err = client.Call(method, name)
	assert.Equal(t, nil, err)

	method, name = RpcArgs("start", "vcap", true)
	_, err = client.Call(method, name)
	assert.Equal(t, nil, err)

	method, name = RpcArgs("start", "bar", true)
	_, err = client.Call(method, name)
	assert.NotEqual(t, nil, err)

	method, name = RpcArgs("about", "", false)
	reply, err := client.Call(method, name)
	assert.Equal(t, nil, err)

	about, ok := reply.(*About)
	assert.Equal(t, true, ok)
	assert.Equal(t, VERSION, about.Version)

	reply, err = client.Call("ENOENT", "")
	assert.NotEqual(t, nil, err)
}

func TestRemote(t *testing.T) {
	err := helper.WithRpcServer(func(c *rpc.Client) {
		client := NewRemoteClient(c, mockAPI)
		runTests(t, client)
	})

	assert.Equal(t, nil, err)
}

func TestLocal(t *testing.T) {
	client := NewLocalClient(mockAPI)
	runTests(t, client)
}
