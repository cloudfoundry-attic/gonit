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
	verbose           = flag.Bool("int.v", false, "Verbose integration output.")
	integrationDir    string
	gonitMainDir      string
	integrationTmpDir string
)

func setup() {
	flag.Parse()

	if intDir, err := os.Getwd(); err != nil {
		log.Printf("Error getting pwd: %v\n", err.Error())
		os.Exit(2)
	} else {
		integrationDir = intDir
	}

	integrationTmpDir := integrationDir+"/tmp"
	gonitMainDir = integrationDir + "/../../gonit"

	if err := os.MkdirAll(integrationTmpDir+"/process_tmp", 0755); err != nil {
		log.Printf(err.Error())
		os.Exit(2)
	}

	err := helper.BuildBin(integrationDir+"/../process",
		integrationTmpDir+"/goprocess")
	if err != nil {
		log.Printf(err.Error())
		os.Exit(2)
	}

	err = helper.BuildBin(gonitMainDir, integrationTmpDir+"/gonit")
	if err != nil {
		log.Printf(err.Error())
		os.Exit(2)
	}

	if err := os.Chdir(gonitMainDir); err != nil {
		log.Printf("Error changing dir: %v\n", err.Error())
		os.Exit(2)
	}
}

func cleanup() {
	if err := os.RemoveAll(integrationDir + "/tmp"); err != nil {
		log.Printf("Error deleting '%v': '%v'\n", integrationTmpDir,
			err.Error())
	}
}

func Test(t *testing.T) {
	setup()
	TestingT(t)
	cleanup()
}
