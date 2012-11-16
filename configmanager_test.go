// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"io/ioutil"
	. "launchpad.net/gocheck"
	"os"
)

type ConfigSuite struct{}

var _ = Suite(&ConfigSuite{})

func assertFileParsed(c *C, configManager *ConfigManager) {
	c.Check(1, Equals, len(configManager.ProcessGroups))
	pg := configManager.ProcessGroups["dashboard"]
	c.Check(ProcessGroup{}, Not(Equals), pg)
	c.Check("dashboard", Equals, pg.Name)

	c.Check(4, Equals, len(pg.Events))
	opentsdb := pg.Processes["opentsdb"]
	c.Check(Process{}, Not(Equals), opentsdb)
	dashboard := pg.Processes["dashboard"]
	c.Check(Process{}, Not(Equals), dashboard)
	c.Check(2, Equals, len(opentsdb.Actions["alert"]))
	c.Check(1, Equals, len(opentsdb.Actions["restart"]))
	c.Check(1, Equals, len(dashboard.Actions["alert"]))
	c.Check("memory_used > 5mb", Equals, pg.EventByName("memory_over_5").Rule)
	c.Check((*Event)(nil), Equals, pg.EventByName("blah"))

	c.Check("none", Equals, configManager.Settings.AlertTransport)
	c.Check("", Not(Equals), configManager.Settings.RpcServerUrl)
	c.Check(0, Equals, configManager.Settings.ProcessPollInterval)
	c.Check(configManager.Settings.Daemon, NotNil)
	c.Check("lolnit", Equals, configManager.Settings.Daemon.Name)
	c.Check(configManager.Settings.Logging, NotNil)
	c.Check("debug", Equals, configManager.Settings.Logging.Level)
}

func (s *ConfigSuite) TestGetPid(c *C) {
	file, err := ioutil.TempFile("", "pid")
	if err != nil {
		c.Error(err)
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err := file.Write([]byte("1234")); err != nil {
		c.Fatal(err)
	}
	process := Process{Pidfile: file.Name()}
	process.Pidfile = file.Name()
	pid, err := process.Pid()
	if err != nil {
		c.Fatal(err)
	}
	c.Check(1234, Equals, pid)
}

func (s *ConfigSuite) TestParseDir(c *C) {
	configManager := &ConfigManager{}
	err := configManager.LoadConfig("test/config/")
	if err != nil {
		c.Fatal(err)
	}
	assertFileParsed(c, configManager)
}

func (s *ConfigSuite) TestNoSettingsLoadsDefaults(c *C) {
	configManager := &ConfigManager{}
	err := configManager.LoadConfig("test/config/dashboard-gonit.yml")
	if err != nil {
		c.Fatal(err)
	}
	c.Check("none", Equals, configManager.Settings.AlertTransport)
}

func (s *ConfigSuite) TestLoadBadDir(c *C) {
	configManager := &ConfigManager{}
	err := configManager.LoadConfig("Bad/Dir")
	c.Check(err, NotNil)
	c.Check("Error stating path 'Bad/Dir'.", Equals, err.Error())
}

func (s *ConfigSuite) TestRequiredFieldsExist(c *C) {
	process := &Process{}
	processes := map[string]*Process{}
	processes["foobar"] = process
	pg := ProcessGroup{Processes: processes}
	processErr := "foobar must have name, description, pidfile and start."
	eventsErr := "some_event must have name, description, rule, and actions."

	err := pg.validateRequiredFieldsExist()
	c.Check(processErr, Equals, err.Error())

	process.Name = "foobar"
	err = pg.validateRequiredFieldsExist()
	c.Check(processErr, Equals, err.Error())

	process.Description = "Some description."
	err = pg.validateRequiredFieldsExist()
	c.Check(processErr, Equals, err.Error())

	process.Pidfile = "pidfile"
	err = pg.validateRequiredFieldsExist()
	c.Check(processErr, Equals, err.Error())

	process.Start = "startscript"
	err = pg.validateRequiredFieldsExist()
	c.Check(nil, IsNil)

	event := &Event{}
	events := map[string]*Event{}
	events["some_event"] = event
	pg.Events = events

	err = pg.validateRequiredFieldsExist()
	c.Check(eventsErr, Equals, err.Error())

	event.Name = "some_event"
	err = pg.validateRequiredFieldsExist()
	c.Check(eventsErr, Equals, err.Error())

	event.Description = "some description"
	err = pg.validateRequiredFieldsExist()
	c.Check(eventsErr, Equals, err.Error())

	event.Rule = "some rule"
	err = pg.validateRequiredFieldsExist()
	c.Check(nil, IsNil)
}

func (s *ConfigSuite) TestValidatePersistErr(c *C) {
	settings := &Settings{PersistFile: "/does/not/exist"}
	err := settings.validatePersistFile()
	c.Check(err, NotNil)
}

func (s *ConfigSuite) TestValidatePersistGood(c *C) {
	persistFile := os.Getenv("PWD") + "/test/config/expected_persist_file.yml"
	settings := &Settings{PersistFile: persistFile}
	err := settings.validatePersistFile()
	c.Check(err, IsNil)
}
