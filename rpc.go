// Copyright (c) 2012 VMware, Inc.

// Wrapper for RPC server configuration and lifecycle

package gonit

import (
	"fmt"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"net/url"
	"os"
)

var info = &RpcServerInfo{
	Version: VERSION,
}

func init() {
	rpc.Register(info)
}

type RpcServerInfo struct {
	Accepts uint64
	Version string
}

func (self *RpcServerInfo) Get(args interface{}, reply *RpcServerInfo) error {
	*reply = *self
	return nil
}

type RpcServer struct {
	listener net.Listener
	cleanup  func()
}

func NewRpcServer(listenURL string) (*RpcServer, error) {
	var listener net.Listener
	var cleanup func()

	url, err := url.Parse(listenURL)
	if err != nil {
		return nil, err
	}

	switch url.Scheme {
	case "tcp":
		if url.Host == "" {
			err = fmt.Errorf("Invalid URL %q", listenURL)
		} else {
			listener, err = net.Listen("tcp", url.Host)
		}
	case "", "unix":
		listener, err = net.Listen("unix", url.Path)
		cleanup = func() { os.Remove(url.Path) }
	default:
		err = net.UnknownNetworkError(url.Scheme)
	}

	if err != nil {
		return nil, err
	}

	server := &RpcServer{
		listener: listener,
		cleanup:  cleanup,
	}

	return server, nil
}

func (self *RpcServer) Shutdown() {
	self.listener.Close()
	if self.cleanup != nil {
		self.cleanup()
	}
	info.Accepts = 0 // reset counter
}

func (self *RpcServer) Serve() error {
	defer self.Shutdown()

	for {
		conn, err := self.listener.Accept()

		if err != nil {
			return err
		}

		info.Accepts++

		go jsonrpc.ServeConn(conn)
	}

	panic("not reached")
}
