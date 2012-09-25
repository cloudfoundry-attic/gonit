// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"github.com/bmizerany/assert"
	"io/ioutil"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"testing"
)

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

func TestUnixServer(t *testing.T) {
	file, err := ioutil.TempFile("", "gonit-rpc")
	if err != nil {
		t.Error(err)
	}
	path := file.Name()
	defer os.Remove(path)

	url := "unix://" + path

	server, err := NewRpcServer(url)

	assert.Equal(t, nil, err)

	// check file exists and is a unix socket
	fi, err := os.Stat(path)
	assert.Equal(t, nil, err)
	assert.Equal(t, os.ModeSocket, fi.Mode()&os.ModeSocket)

	// start the server in a goroutine (Accept blocks)
	go server.Serve()

	info.Requests = 0
	// make some client rpcs
	for i := 0; i <= 2; i++ {
		client, err := jsonrpc.Dial("unix", path)
		assert.Equal(t, nil, err)

		info := &RpcTest{}

		for j := 1; j < 2; j++ {
			err = client.Call("RpcTest.Info", nil, info)
			assert.Equal(t, nil, err)

			assert.Equal(t, uint64(i+j), info.Requests)
			assert.Equal(t, VERSION, info.Version)
		}

		client.Close()
	}

	// shutdown + cleanup
	server.Shutdown()

	fi, err = os.Stat(path) // should be gone after Shutdown()
	assert.NotEqual(t, nil, err)
}

func TestTcpServer(t *testing.T) {
	host := "localhost:9999"
	url := "tcp://" + host

	server, err := NewRpcServer(url)
	assert.Equal(t, nil, err)

	// start the server in a goroutine (Accept blocks)
	go server.Serve()

	info.Requests = 0

	client, err := jsonrpc.Dial("tcp", host)
	assert.Equal(t, nil, err)

	info := &RpcTest{}

	err = client.Call("RpcTest.Info", nil, info)
	assert.Equal(t, uint64(1), info.Requests)
	assert.Equal(t, VERSION, info.Version)
	client.Close()

	// shutdown + cleanup
	server.Shutdown()
}

func TestInvalidURL(t *testing.T) {
	_, err := NewRpcServer("no://way") // unsupported scheme
	assert.NotEqual(t, nil, err)
	_, ok := err.(*net.UnknownNetworkError)
	assert.NotEqual(t, true, ok)

	_, err = NewRpcServer("/") // directory
	assert.NotEqual(t, nil, err)

	_, err = NewRpcServer("tcp:whoops") // wups
	assert.NotEqual(t, nil, err)
}
