// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"fmt"
	"net/rpc"
	"reflect"
	"unicode"
)

// API interface for cli commands.
// When gonit is running as a daemon, will be RPCs;
// Otherwise, invoke the API in-process via reflection.
type CliClient interface {
	Call(action string, name string) (interface{}, error)
	Close() error
}

// RPC client implementation
type remoteClient struct {
	client *rpc.Client
	rcvr   interface{}
}

// In-process client implementation
type localClient struct {
	rcvr interface{}
}

// Dispatch method via RPC
func (c *remoteClient) Call(action string, name string) (interface{}, error) {
	method, err := lookupRpcMethod(c.rcvr, action)
	if err != nil {
		return nil, err
	}
	reply := newRpcReply(method)

	service := reflect.TypeOf(c.rcvr).Elem().Name() + "." + method.Name
	err = c.client.Call(service, name, reply.Interface())

	return reply.Interface(), err
}

// Close RPC client
func (c *remoteClient) Close() error {
	return c.client.Close()
}

// Dispatch method via reflection
func (c *localClient) Call(action string, name string) (interface{}, error) {
	method, err := lookupRpcMethod(c.rcvr, action)
	if err != nil {
		return nil, err
	}
	reply := newRpcReply(method)

	params := []reflect.Value{
		reflect.ValueOf(c.rcvr),
		reflect.ValueOf(name),
		reply,
	}

	returnValues := method.Func.Call(params)

	errorInterface := returnValues[0].Interface()
	if errorInterface != nil {
		err = errorInterface.(error)
	}

	return reply.Interface(), err
}

// noop
func (c *localClient) Close() error {
	return nil
}

func NewRemoteClient(client *rpc.Client, rcvr interface{}) CliClient {
	return &remoteClient{client: client, rcvr: rcvr}
}

func NewLocalClient(rcvr interface{}) CliClient {
	return &localClient{rcvr: rcvr}
}

// helper to lookup api method via name string
func lookupRpcMethod(rcvr interface{}, name string) (reflect.Method, error) {
	typ := reflect.TypeOf(rcvr)
	method, ok := typ.MethodByName(name)
	if ok {
		return method, nil
	}

	return method, fmt.Errorf("unknown method %q", name)
}

// helper to allocate RPC reply struct
func newRpcReply(method reflect.Method) reflect.Value {
	mtype := method.Type
	return reflect.New(mtype.In(2).Elem())
}

// helper to convert command-line arguments to api method call
func RpcArgs(method string, name string, isGroup bool) (string, string) {
	if name != "" {
		var kind string

		if isGroup {
			kind = "Group"
		} else {
			kind = "Process"
		}

		if name == "all" {
			kind = "All"
			name = ""
		}

		method = method + kind
	} else if method == "status" {
		// Default `gonit status` to `gonit status all`
		method = method + "All"
	}

	return ucfirst(method), name
}

// cli args are lower-case, exported/public RPC methods start with upper-case
func ucfirst(s string) string {
	r := []rune(s)
	return string(unicode.ToUpper(r[0])) + string(r[1:])
}
