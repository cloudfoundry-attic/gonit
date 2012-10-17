// Copyright (c) 2012 VMware, Inc.

package gonit_test

import (
	"fmt"
	. "github.com/cloudfoundry/gonit"
	"github.com/cloudfoundry/gonit/test/helper"
	. "launchpad.net/gocheck"
	"net/rpc"
)

type CliSuite struct{}

var _ = Suite(&CliSuite{})

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
func runTests(c *C, client CliClient) {
	method, name := RpcArgs("stop", "foo", false)
	_, err := client.Call(method, name)
	c.Check(err, IsNil)

	method, name = RpcArgs("stop", "bar", false)
	_, err = client.Call(method, name)
	c.Check(err, NotNil)

	method, name = RpcArgs("unmonitor", "all", false)
	_, err = client.Call(method, name)
	c.Check(err, IsNil)

	method, name = RpcArgs("start", "vcap", true)
	_, err = client.Call(method, name)
	c.Check(err, IsNil)

	method, name = RpcArgs("start", "bar", true)
	_, err = client.Call(method, name)
	c.Check(err, NotNil)

	method, name = RpcArgs("about", "", false)
	reply, err := client.Call(method, name)
	c.Check(err, IsNil)

	about, ok := reply.(*About)
	c.Check(true, Equals, ok)
	c.Check(VERSION, Equals, about.Version)

	reply, err = client.Call("ENOENT", "")
	c.Check(err, NotNil)
}

func (s *CliSuite) TestRemote(c *C) {
	err := helper.WithRpcServer(func(rc *rpc.Client) {
		client := NewRemoteClient(rc, mockAPI)
		runTests(c, client)
	})

	c.Check(err, IsNil)
}

func (s *CliSuite) TestLocal(c *C) {
	client := NewLocalClient(mockAPI)
	runTests(c, client)
}
