// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"github.com/bmizerany/assert"
	"io/ioutil"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"testing"
)

var rpcName = "TestAPI"

func init() {
	rpc.RegisterName(rpcName, &API{})
}

func withRpcServer(f func(c *rpc.Client)) error {
	file, err := ioutil.TempFile("", "gonit-rpc")
	if err != nil {
		return err
	}
	path := file.Name()
	defer os.Remove(path)
	os.Remove(path)

	url := "unix://" + path

	server, err := NewRpcServer(url)

	if err != nil {
		return err
	}

	go server.Serve()

	client, err := jsonrpc.Dial("unix", path)

	if err != nil {
		return err
	}

	defer client.Close()

	f(client)

	server.Shutdown()

	return nil
}

func TestStartAll(t *testing.T) {
	err := withRpcServer(func(c *rpc.Client) {
		reply := ActionResult{}
		err := c.Call(rpcName+".StartAll", "", &reply)
		assert.Equal(t, notimpl.Error(), err.Error())
	})

	assert.Equal(t, nil, err)
}

func TestAbout(t *testing.T) {
	err := withRpcServer(func(c *rpc.Client) {
		params := struct{}{}
		about := About{}
		err := c.Call(rpcName+".About", &params, &about)
		assert.Equal(t, nil, err)

		assert.Equal(t, VERSION, about.Version)

		// need not be same type, just requires same fields
		var gabout struct {
			Version string
		}

		err = c.Call(rpcName+".About", &params, &gabout)
		assert.Equal(t, nil, err)

		assert.Equal(t, VERSION, gabout.Version)
	})

	assert.Equal(t, nil, err)
}
