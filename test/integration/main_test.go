// Copyright (c) 2012 VMware, Inc.

package gonit_integration

import (
	"flag"
	"github.com/cloudfoundry/gonit/test/helper"
	. "launchpad.net/gocheck"
	"log"
	"os"
	"testing"
)

var (
	integrationDir    string
	gonitMainDir      string
	integrationTmpDir string
)

func setup() {
	flag.Parse()

	if intDir, err := os.Getwd(); err != nil {
		log.Panicf("Error getting pwd: %v", err.Error())
	} else {
		integrationDir = intDir
	}

	integrationTmpDir := integrationDir + "/tmp"
	gonitMainDir = integrationDir + "/../../gonit"

	if err := os.MkdirAll(integrationTmpDir, 0755); err != nil {
		log.Panicf(err.Error())
	}

	err := helper.BuildBin(integrationDir+"/../process",
		integrationTmpDir+"/goprocess")
	if err != nil {
		log.Panicf(err.Error())
	}

	err = helper.BuildBin(gonitMainDir, integrationTmpDir+"/gonit")
	if err != nil {
		log.Panicf(err.Error())
	}

	if err := os.Chdir(integrationTmpDir); err != nil {
		log.Panicf("Error changing dir: %v", err.Error())
	}
}

func cleanup() {
	if err := os.RemoveAll(integrationDir + "/tmp"); err != nil {
		log.Printf("Error deleting '%v': '%v'", integrationTmpDir, err.Error())
	}
}

func Test(t *testing.T) {
	setup()
	TestingT(t)
	cleanup()
}
