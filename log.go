// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"github.com/cloudfoundry/gosteno"
	"os"
)

var Log steno.Logger

type LoggerConfig struct {
	Level    string
	FileName string
	Codec    string
	file     *os.File
}

func init() {
	// default configuration
	config := &LoggerConfig{}
	err := config.Init()
	if err != nil {
		panic(err)
	}
}

func (lc *LoggerConfig) Init() error {
	var file *os.File
	var level steno.LogLevel
	var err error

	if lc.Level != "" {
		level, err = steno.GetLogLevel(lc.Level)
		if err != nil {
			return err
		}
	}

	if lc.FileName == "" {
		file = os.Stdout
	} else {
		flags := os.O_APPEND | os.O_CREATE | os.O_WRONLY
		lc.file, err = os.OpenFile(lc.FileName, flags, 0666)
		if err != nil {
			return err
		}
		file = lc.file
	}

	out := steno.NewIOSink(file)

	if lc.Codec != "json" {
		out.SetCodec(steno.NewJsonPrettifier(steno.EXCLUDE_DATA))
	}

	steno.Init(&steno.Config{
		EnableLOC: true,
		Sinks:     []steno.Sink{out},
		Level:     level,
	})

	Log = steno.NewLogger("gonit")

	return nil
}

func (lc *LoggerConfig) Close() error {
	if lc.file != nil {
		return lc.file.Close()
	}
	return nil
}
