// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"github.com/bmizerany/assert"
	"io/ioutil"
	"os"
	"testing"
)

func assertFileParsed(t *testing.T, configManager *ConfigManager) {
	assert.Equal(t, 1, len(configManager.ProcessGroups))
	pg := configManager.ProcessGroups["dashboard"]
	assert.NotEqual(t, ProcessGroup{}, pg)
	assert.Equal(t, "dashboard", pg.Name)

	assert.Equal(t, 4, len(pg.Events))
	opentsdb := pg.Processes["opentsdb"]
	assert.NotEqual(t, Process{}, opentsdb)
	dashboard := pg.Processes["dashboard"]
	assert.NotEqual(t, Process{}, dashboard)
	assert.Equal(t, 2, len(opentsdb.Actions["alert"]))
	assert.Equal(t, 1, len(opentsdb.Actions["restart"]))
	assert.Equal(t, 1, len(dashboard.Actions["alert"]))
	assert.Equal(t, "memory_used > 5mb", pg.EventByName("memory_over_5").Rule)
	assert.Equal(t, (*Event)(nil), pg.EventByName("blah"))

	assert.Equal(t, "none", configManager.Settings.AlertTransport)
	assert.NotEqual(t, "", configManager.Settings.RpcServerUrl)
	assert.Equal(t, 0, configManager.Settings.PollInterval)
	assert.NotEqual(t, nil, configManager.Settings.Daemon)
	assert.Equal(t, "lolnit", configManager.Settings.Daemon.Name)
	assert.NotEqual(t, nil, configManager.Settings.Logging)
	assert.Equal(t, "debug", configManager.Settings.Logging.Level)
}

func TestGetPid(t *testing.T) {
	file, err := ioutil.TempFile("", "pid")
	if err != nil {
		t.Error(err)
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err := file.Write([]byte("1234")); err != nil {
		t.Fatal(err)
	}
	process := Process{Pidfile: file.Name()}
	process.Pidfile = file.Name()
	pid, err := process.Pid()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1234, pid)
}

func TestParseDir(t *testing.T) {
	configManager := ConfigManager{}
	err := configManager.Parse("test/config/")
	if err != nil {
		t.Fatal(err)
	}
	assertFileParsed(t, &configManager)
}

func TestParseFileList(t *testing.T) {
	configManager := ConfigManager{}
	err := configManager.Parse("test/config/dashboard-gonit.yml",
		"test/config/gonit.yml")
	if err != nil {
		t.Fatal(err)
	}
	assertFileParsed(t, &configManager)
}

func TestNoSettingsLoadsDefaults(t *testing.T) {
	configManager := ConfigManager{}
	err := configManager.Parse("test/config/dashboard-gonit.yml")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "none", configManager.Settings.AlertTransport)
}

func TestLoadBadDir(t *testing.T) {
	configManager := ConfigManager{}
	err := configManager.Parse("Bad/Dir")
	assert.NotEqual(t, nil, err)
	assert.Equal(t, "Error stating path 'Bad/Dir'.\n", err.Error())
}

func TestRequiredFieldsExist(t *testing.T) {
	process := &Process{}
	processes := map[string]*Process{}
	processes["foobar"] = process
	pg := ProcessGroup{Processes: processes}
	processErr := "foobar must have name, description, pidfile and start."
	eventsErr := "some_event must have name, description, rule, and actions."

	err := pg.validateRequiredFieldsExist()
	assert.Equal(t, processErr, err.Error())

	process.Name = "foobar"
	err = pg.validateRequiredFieldsExist()
	assert.Equal(t, processErr, err.Error())

	process.Description = "Some description."
	err = pg.validateRequiredFieldsExist()
	assert.Equal(t, processErr, err.Error())

	process.Pidfile = "pidfile"
	err = pg.validateRequiredFieldsExist()
	assert.Equal(t, processErr, err.Error())

	process.Start = "startscript"
	err = pg.validateRequiredFieldsExist()
	assert.Equal(t, nil, nil)

	event := &Event{}
	events := map[string]*Event{}
	events["some_event"] = event
	pg.Events = events

	err = pg.validateRequiredFieldsExist()
	assert.Equal(t, eventsErr, err.Error())

	event.Name = "some_event"
	err = pg.validateRequiredFieldsExist()
	assert.Equal(t, eventsErr, err.Error())

	event.Description = "some description"
	err = pg.validateRequiredFieldsExist()
	assert.Equal(t, eventsErr, err.Error())

	event.Rule = "some rule"
	err = pg.validateRequiredFieldsExist()
	assert.Equal(t, nil, nil)
}
