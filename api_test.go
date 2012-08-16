// Copyright (c) 2012 VMware, Inc.

package gonit_test

import (
	"github.com/bmizerany/assert"
	. "github.com/cloudfoundry/gonit"
	"github.com/cloudfoundry/gonit/test/helper"
	"net/rpc"
	"strings"
	"testing"
)

var rpcName = "TestAPI"

func init() {
	rpc.RegisterName(rpcName, NewAPI(nil))
}

func isActionError(err error) bool {
	return strings.HasPrefix(err.Error(), "ActionError")
}

func TestActionsNoConfig(t *testing.T) {
	err := helper.WithRpcServer(func(c *rpc.Client) {
		reply := &ActionResult{}
		err := c.Call(rpcName+".StartAll", "", reply)
		assert.Equal(t, nil, err)
		// Total == 0 since no processes are configured
		assert.Equal(t, 0, reply.Total)
		assert.Equal(t, 0, reply.Errors)

		reply = &ActionResult{}
		err = c.Call(rpcName+".StartProcess", "foo", reply)
		assert.NotEqual(t, nil, err)
		assert.Equal(t, true, isActionError(err))

		reply = &ActionResult{}
		err = c.Call(rpcName+".StartGroup", "bar", reply)
		assert.NotEqual(t, nil, err)
		assert.Equal(t, true, isActionError(err))
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
