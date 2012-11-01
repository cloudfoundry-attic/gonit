// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"io/ioutil"
	. "launchpad.net/gocheck"
	"os"
)

func reInitLogger() {
	(&LoggerConfig{}).Init() // reset to defaults
}

type LogSuite struct{}

var _ = Suite(&LogSuite{})

func (s *LogSuite) TestLogInit(c *C) {
	defer reInitLogger()

	config := &LoggerConfig{
		Level: "debug",
	}

	err := config.Init()
	c.Check(err, IsNil)
	c.Check(config.file, IsNil)
	err = config.Close()
	c.Check(err, IsNil)
}

func (s *LogSuite) TestInvalidLogLevel(c *C) {
	defer reInitLogger()

	config := &LoggerConfig{
		Level: "enolevel",
	}

	err := config.Init()
	c.Check(err, NotNil)
}

func (s *LogSuite) TestLogFile(c *C) {
	defer reInitLogger()

	file, err := ioutil.TempFile("", "gonit_log")
	c.Check(err, IsNil)
	defer os.Remove(file.Name())

	config := &LoggerConfig{
		FileName: file.Name(),
		Level:    "info",
	}

	err = config.Init()
	c.Check(err, IsNil)

	fi, err := os.Stat(config.FileName)
	c.Check(err, IsNil)
	c.Check(fi.Size(), Equals, int64(0))

	// info message should be written to the log file
	Log.Info("testing")
	fi, err = os.Stat(config.FileName)
	c.Check(err, IsNil)
	c.Check(fi.Size(), Not(Equals), int64(0))

	// info message should not
	Log.Debug("another test")
	fi2, err := os.Stat(config.FileName)
	c.Check(err, IsNil)
	c.Check(fi.Size(), Equals, fi2.Size())

	err = config.Close()
	c.Check(err, IsNil)

	// check Init returns an error when file create fails
	config.FileName = "/dev/null/gonnafail"
	err = config.Init()
	c.Check(err, NotNil)
}
