// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"io/ioutil"
	. "launchpad.net/gocheck"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
)

type RpcSuite struct{}

var _ = Suite(&RpcSuite{})

// simple RPC interface just for testing
var info = &RpcTest{
	Version: VERSION,
}

func init() {
	rpc.Register(info)
}

type RpcTest struct {
	Requests uint64
	Version  string
}

func (info *RpcTest) Info(args interface{}, reply *RpcTest) error {
	info.Requests++
	*reply = *info
	return nil
}

func (s *RpcSuite) UnixServer(c *C) {
	file, err := ioutil.TempFile("", "gonit-rpc")
	if err != nil {
		c.Error(err)
	}
	path := file.Name()
	defer os.Remove(path)

	url := "unix://" + path

	server, err := NewRpcServer(url)

	c.Check(err, IsNil)

	// check file exists and is a unix socket
	fi, err := os.Stat(path)
	c.Check(err, IsNil)
	c.Check(os.ModeSocket, Equals, fi.Mode()&os.ModeSocket)

	// start the server in a goroutine (Accept blocks)
	go server.Serve()

	info.Requests = 0
	// make some client rpcs
	for i := 0; i <= 2; i++ {
		client, err := jsonrpc.Dial("unix", path)
		c.Check(err, IsNil)

		info := &RpcTest{}

		for j := 1; j < 2; j++ {
			err = client.Call("RpcTest.Info", nil, info)
			c.Check(err, IsNil)

			c.Check(uint64(i+j), Equals, info.Requests)
			c.Check(VERSION, Equals, info.Version)
		}

		client.Close()
	}

	// shutdown + cleanup
	server.Shutdown()

	fi, err = os.Stat(path) // should be gone after Shutdown()
	c.Check(err, NotNil)
}

func (s *RpcSuite) TcpServer(c *C) {
	host := "localhost:9999"
	url := "tcp://" + host

	server, err := NewRpcServer(url)
	c.Check(err, IsNil)

	// start the server in a goroutine (Accept blocks)
	go server.Serve()

	info.Requests = 0

	client, err := jsonrpc.Dial("tcp", host)
	c.Check(err, IsNil)

	info := &RpcTest{}

	err = client.Call("RpcTest.Info", nil, info)
	c.Check(uint64(1), Equals, info.Requests)
	c.Check(VERSION, Equals, info.Version)
	client.Close()

	// shutdown + cleanup
	server.Shutdown()
}

func (s *RpcSuite) InvalidURL(c *C) {
	_, err := NewRpcServer("no://way") // unsupported scheme
	c.Check(err, NotNil)
	_, ok := err.(*net.UnknownNetworkError)
	c.Check(true, Not(Equals), ok)

	_, err = NewRpcServer("/") // directory
	c.Check(err, NotNil)

	_, err = NewRpcServer("tcp:whoops") // wups
	c.Check(err, NotNil)
}
