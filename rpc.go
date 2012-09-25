// Copyright (c) 2012 VMware, Inc.

// Wrapper for RPC server configuration and lifecycle

package gonit

import (
	"fmt"
	"net"
	"net/rpc/jsonrpc"
	"net/url"
	"os"
)

type RpcServer struct {
	listener net.Listener
	cleanup  func()
}

// Construct a new RpcServer via string URL
// Currently supporting json RPC over unix or tcp socket
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
		// If the file already exists, we need to remove it otherwise gonit can't
		// start.
		os.Remove(url.Path)
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

// Close listener socket
// In the case of unix socket, remove socket file.
func (s *RpcServer) Shutdown() {
	s.listener.Close()
	if s.cleanup != nil {
		s.cleanup()
	}
}

// Accept unix|tcp connections and serve json RPCs
func (s *RpcServer) Serve() error {
	defer s.Shutdown()

	for {
		conn, err := s.listener.Accept()

		if err != nil {
			return err
		}

		go jsonrpc.ServeConn(conn)
	}

	panic("not reached")
}
