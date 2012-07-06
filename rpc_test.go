// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"github.com/bmizerany/assert"
	"io/ioutil"
	"net"
	"net/rpc/jsonrpc"
	"os"
	"syscall"
	"testing"
)

func TestUnixServer(t *testing.T) {
	file, err := ioutil.TempFile("", "gonit-rpc")
	if err != nil {
		t.Error(err)
	}
	path := file.Name()
	defer os.Remove(path)

	url := "unix://" + path

	server, err := NewRpcServer(url)

	// expected when file already exists
	assert.NotEqual(t, nil, err)
	assert.Equal(t, syscall.EADDRINUSE, err.(*net.OpError).Err)

	os.Remove(path)
	fi, err := os.Stat(path) // double check file is gone
	assert.NotEqual(t, nil, err)

	server, err = NewRpcServer(url)
	assert.Equal(t, nil, err)

	// check file exists and is a unix socket
	fi, err = os.Stat(path)
	assert.Equal(t, nil, err)
	assert.Equal(t, os.ModeSocket, fi.Mode()&os.ModeSocket)

	// start the server in a goroutine (Accept blocks)
	go server.Serve()

	// make some client rpcs
	for i := 1; i <= 2; i++ {
		client, err := jsonrpc.Dial("unix", path)
		assert.Equal(t, nil, err)

		info := &RpcServerInfo{}

		for j := 0; j < 3; j++ {
			err = client.Call("RpcServerInfo.Get", nil, info)
			assert.Equal(t, nil, err)

			// Accepts increments per-connection, not per rpc
			assert.Equal(t, uint64(i), info.Accepts)
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

	client, err := jsonrpc.Dial("tcp", host)
	assert.Equal(t, nil, err)

	info := &RpcServerInfo{}

	err = client.Call("RpcServerInfo.Get", nil, info)
	assert.Equal(t, uint64(1), info.Accepts)
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
