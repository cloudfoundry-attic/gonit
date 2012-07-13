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

	rv := method.Func.Call(params)

	ei := rv[0].Interface()
	if ei != nil {
		err = ei.(error)
	}

	return reply.Interface(), err
}

// noop
func (c *localClient) Close() error {
	return nil
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
func rpcArgs(method string, name string, isGroup bool) (string, string) {
	if name != "" {
		var kind string

		if isGroup {
			kind = "Group"
		} else {
			kind = "Service"
		}

		if name == "all" {
			kind = "All"
			name = ""
		}

		method = method + kind
	}

	return ucfirst(method), name
}

// cli args are lower-case, exported/public RPC methods start with upper-case
func ucfirst(s string) string {
	r := []rune(s)
	return string(unicode.ToUpper(r[0])) + string(r[1:])
}
