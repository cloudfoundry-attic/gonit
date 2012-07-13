// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"fmt"
	"github.com/bmizerany/assert"
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

func (*MockAPI) StopService(name string, r *ActionResult) error {
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
	method, name := rpcArgs("stop", "foo", false)
	reply, err := client.Call(method, name)
	assert.Equal(t, nil, err)

	method, name = rpcArgs("stop", "bar", false)
	reply, err = client.Call(method, name)
	assert.NotEqual(t, nil, err)

	method, name = rpcArgs("unmonitor", "all", false)
	reply, err = client.Call(method, name)
	assert.Equal(t, nil, err)

	method, name = rpcArgs("start", "vcap", true)
	reply, err = client.Call(method, name)
	assert.Equal(t, nil, err)

	method, name = rpcArgs("start", "bar", true)
	reply, err = client.Call(method, name)
	assert.NotEqual(t, nil, err)

	method, name = rpcArgs("about", "", false)
	reply, err = client.Call(method, name)
	assert.Equal(t, nil, err)

	about, ok := reply.(*About)
	assert.Equal(t, true, ok)
	assert.Equal(t, VERSION, about.Version)

	reply, err = client.Call("ENOENT", "")
	assert.NotEqual(t, nil, err)
}

func TestRemote(t *testing.T) {
	err := withRpcServer(func(c *rpc.Client) {
		client := &remoteClient{client: c, rcvr: mockAPI}
		runTests(t, client)
	})

	assert.Equal(t, nil, err)
}

func TestLocal(t *testing.T) {
	client := &localClient{rcvr: mockAPI}
	runTests(t, client)
}
