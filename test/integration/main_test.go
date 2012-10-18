// Copyright (c) 2012 VMware, Inc.

package gonit_integration_test

import (
	"flag"
	"fmt"
	"github.com/cloudfoundry/gonit/test/helper"
	. "launchpad.net/gocheck"
	"os"
	"testing"
)

var (
	verbose        = false
	integrationDir string
	gonitMainDir   string
)

func Test(t *testing.T) {
	// propagate 'go test -v' to '-gocheck.v'
	if flag.Lookup("test.v").Value.String() == "true" {
		verbose = true
		flag.Lookup("gocheck.v").Value.Set("true")
	}
	flag.Parse()
	if intDir, err := os.Getwd(); err != nil {
		fmt.Printf("Error getting pwd: %v", err.Error())
		os.Exit(2)
	} else {
		integrationDir = intDir
	}
	gonitMainDir = integrationDir + "/../../gonit"
	helper.BuildBin(integrationDir + "/../process", integrationDir + "/goprocess")
	helper.BuildBin(gonitMainDir, "")
	helper.ChDirOrExit(gonitMainDir)
	TestingT(t)
	// TODO: Cleanup .pid and .json file.
}
