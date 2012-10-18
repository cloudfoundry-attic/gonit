// Copyright (c) 2012 VMware, Inc.

package gonit_integration

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
	if flag.Lookup("gocheck.vv").Value.String() == "true" {
		verbose = true
	}
	flag.Parse()
	if intDir, err := os.Getwd(); err != nil {
		fmt.Printf("Error getting pwd: %v", err.Error())
		os.Exit(2)
	} else {
		integrationDir = intDir
	}
	gonitMainDir = integrationDir + "/../../gonit"
	helper.BuildBin(integrationDir+"/../process", integrationDir+"/goprocess")
	helper.BuildBin(gonitMainDir, "")
	helper.ChDirOrExit(gonitMainDir)
	TestingT(t)
	// TODO(lisbakke): Clean up pid and json files after test.
}
