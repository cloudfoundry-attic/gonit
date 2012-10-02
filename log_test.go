// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"github.com/bmizerany/assert"
	"io/ioutil"
	"os"
	"testing"
)

func reInitLogger() {
	(&LoggerConfig{}).Init() // reset to defaults
}

func TestLogInit(t *testing.T) {
	defer reInitLogger()

	config := &LoggerConfig{
		Level: "debug",
	}

	err := config.Init()
	assert.Equal(t, nil, err)
	var empty *os.File
	assert.Equal(t, empty, config.file)
	err = config.Close()
	assert.Equal(t, nil, err)
}

func TestInvalidLogLevel(t *testing.T) {
	defer reInitLogger()

	config := &LoggerConfig{
		Level: "enolevel",
	}

	err := config.Init()
	assert.NotEqual(t, nil, err)
}

func TestLogFile(t *testing.T) {
	defer reInitLogger()

	file, err := ioutil.TempFile("", "gonit_log")
	assert.Equal(t, nil, err)
	defer os.Remove(file.Name())

	config := &LoggerConfig{
		FileName: file.Name(),
		Level:    "info",
	}

	err = config.Init()
	assert.Equal(t, nil, err)

	fi, err := os.Stat(config.FileName)
	assert.Equal(t, nil, err)
	assert.Equal(t, fi.Size(), int64(0))

	// info message should be written to the log file
	Log.Info("testing")
	fi, err = os.Stat(config.FileName)
	assert.Equal(t, nil, err)
	assert.NotEqual(t, fi.Size(), int64(0))

	// info message should not
	Log.Debug("another test")
	fi2, err := os.Stat(config.FileName)
	assert.Equal(t, nil, err)
	assert.Equal(t, fi.Size(), fi2.Size())

	err = config.Close()
	assert.Equal(t, nil, err)

	// make log file read-only and check Init returns an error
	err = os.Chmod(config.FileName, 0444)
	assert.Equal(t, nil, err)
	err = config.Init()
	assert.NotEqual(t, nil, err)
}
