// Copyright (c) 2012 VMware, Inc.

package gonit_test

import (
	"github.com/bmizerany/assert"
	. "github.com/cloudfoundry/gonit"
	"github.com/cloudfoundry/gonit/test/helper"
	"net/rpc"
	"testing"
)

var rpcName = "TestAPI"

func init() {
	rpc.RegisterName(rpcName, &API{})
}

func TestStartAll(t *testing.T) {
	err := helper.WithRpcServer(func(c *rpc.Client) {
		reply := ActionResult{}
		err := c.Call(rpcName+".StartAll", "", &reply)
		assert.NotEqual(t, nil, err)
	})

	assert.Equal(t, nil, err)
}

func TestAbout(t *testing.T) {
	err := helper.WithRpcServer(func(c *rpc.Client) {
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
