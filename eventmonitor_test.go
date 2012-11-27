// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	. "launchpad.net/gocheck"
)

type EventSuite struct{}

var _ = Suite(&EventSuite{})

var eventMonitor EventMonitor

const TWO_GB = uint64(2147483648)

func init() {
	eventMonitor = EventMonitor{}
}

func RegisterNewFakeControl() *FakeControl {
	fc := &FakeControl{}
	eventMonitor.registerControl(fc)
	return fc
}

func (s *EventSuite) TestIsAnOperatorChar(c *C) {
	c.Check(true, Equals, isAnOperatorChar("="))
	c.Check(true, Equals, isAnOperatorChar(">"))
	c.Check(true, Equals, isAnOperatorChar("<"))
	c.Check(false, Equals, isAnOperatorChar("!="))
	c.Check(false, Equals, isAnOperatorChar("=="))
}

func (s *EventSuite) TestCompareUint64(c *C) {
	sixty := uint64(60)
	fiftyNine := uint64(59)
	c.Check(true, Equals, compareUint64(sixty, EQ_OPERATOR, 60))
	c.Check(false, Equals, compareUint64(sixty, EQ_OPERATOR, 61))

	c.Check(true, Equals, compareUint64(sixty, NEQ_OPERATOR, 50))

	c.Check(false, Equals, compareUint64(sixty, LT_OPERATOR, 50))
	c.Check(false, Equals, compareUint64(sixty, LT_OPERATOR, 60))
	c.Check(true, Equals, compareUint64(fiftyNine, LT_OPERATOR, 60))

	c.Check(true, Equals, compareUint64(sixty, GT_OPERATOR, 50))
	c.Check(false, Equals, compareUint64(sixty, GT_OPERATOR, 60))
	c.Check(false, Equals, compareUint64(fiftyNine, GT_OPERATOR, 60))
}

func (s *EventSuite) TestCheckRuleUint(c *C) {
	parsedEvent := &ParsedEvent{
		operator:     EQ_OPERATOR,
		ruleAmount:   uint64(7),
		resourceName: "memory_used",
	}
	resourceVal := uint64(7)
	triggering := checkRule(parsedEvent, resourceVal)
	c.Check(true, Equals, triggering)
}

func (s *EventSuite) TestCheckRuleFalseUint(c *C) {
	parsedEvent := &ParsedEvent{
		operator:     EQ_OPERATOR,
		ruleAmount:   uint64(8),
		resourceName: "memory_used",
	}
	resourceVal := uint64(7)
	triggering := checkRule(parsedEvent, resourceVal)
	c.Check(false, Equals, triggering)
}

func (s *EventSuite) TestParseRuleForwards(c *C) {
	ruleAmount, operator, resourceName, err :=
		eventMonitor.parseRule("memory_used==2gb")
	if err != nil {
		c.Fatal(err)
	}
	c.Check(TWO_GB, Equals, ruleAmount)
	c.Check(EQ_OPERATOR, Equals, operator)
	c.Check("memory_used", Equals, resourceName)
}

func (s *EventSuite) TestParseRuleBackwards(c *C) {
	ruleAmount, operator, resourceName, err :=
		eventMonitor.parseRule("2gb==memory_used")
	if err != nil {
		c.Fatal(err)
	}
	c.Check(TWO_GB, Equals, ruleAmount)
	c.Check(EQ_OPERATOR, Equals, operator)
	c.Check("memory_used", Equals, resourceName)
}

func (s *EventSuite) TestParseRuleSpaces(c *C) {
	ruleAmount, operator, resourceName, err :=
		eventMonitor.parseRule("  2gb   ==  memory_used   ")
	if err != nil {
		c.Fatal(err)
	}
	c.Check(TWO_GB, Equals, ruleAmount)
	c.Check(EQ_OPERATOR, Equals, operator)
	c.Check("memory_used", Equals, resourceName)
}

func (s *EventSuite) TestParseRuleGt(c *C) {
	ruleAmount, operator, resourceName, err :=
		eventMonitor.parseRule("2gb>memory_used")
	if err != nil {
		c.Fatal(err)
	}
	c.Check(TWO_GB, Equals, ruleAmount)
	c.Check(GT_OPERATOR, Equals, operator)
	c.Check("memory_used", Equals, resourceName)
}

func (s *EventSuite) TestParseRuleLt(c *C) {
	ruleAmount, operator, resourceName, err :=
		eventMonitor.parseRule("2gb<memory_used")
	if err != nil {
		c.Fatal(err)
	}
	c.Check(TWO_GB, Equals, ruleAmount)
	c.Check(LT_OPERATOR, Equals, operator)
	c.Check("memory_used", Equals, resourceName)
}

func (s *EventSuite) TestParseRuleInvalidResourceError(c *C) {
	_, _, _, err :=
		eventMonitor.parseRule("2gb<invalid_resource")
	c.Check("Using invalid resource name in rule '2gb<invalid_resource'.", Equals,
		err.Error())

}

func (s *EventSuite) TestParseEvent(c *C) {
	event := Event{
		Rule:        "memory_used>2gb",
		Duration:    "10s",
		Interval:    "10s",
		Description: "The best rule ever!",
	}
	parsedEvent, err :=
		eventMonitor.parseEvent(&event, "GroupName", "ProcessName", "alert")
	if err != nil {
		c.Fatal(err)
	}
	c.Check(GT_OPERATOR, Equals, parsedEvent.operator)
	c.Check("memory_used", Equals, parsedEvent.resourceName)
	c.Check(TWO_GB, Equals, parsedEvent.ruleAmount)
}

func (s *EventSuite) TestParseBadIntervalEvents(c *C) {
	event1 := Event{
		Rule:        "memory_used>2gb",
		Duration:    "8s",
		Interval:    "10s",
		Description: "The best rule ever!",
	}
	event2 := Event{
		Rule:        "cpu_percent>60",
		Duration:    "10s",
		Interval:    "10s",
		Description: "The best rule ever!",
	}
	_, err := eventMonitor.parseEvent(&event1, "GroupName", "ProcessName",
		"alert")
	if err != nil {
		c.Check(
			"Rule 'memory_used>2gb' duration / interval must be an integer.", Equals,

			err.Error())

	}
	_, err = eventMonitor.parseEvent(&event2, "GroupName", "ProcessName", "alert")
	if err != nil {
		c.Check(
			"Rule 'cpu_percent>60' duration / interval must be greater than 1.  It "+
				"is '10 / 10'.", Equals,

			err.Error())

	}
}

type FakeControl struct {
	numDoActionCalled int
	lastActionCalled  int
	isMonitoring      bool
}

func (fc *FakeControl) DoAction(name string, action *ControlAction) error {
	fc.numDoActionCalled++
	fc.lastActionCalled = action.method
	return nil
}

func (fc *FakeControl) IsMonitoring(process *Process) bool {
	return fc.isMonitoring
}

func (s *EventSuite) TestActionTriggers(c *C) {
	fc := RegisterNewFakeControl()
	fc.isMonitoring = true
	eventMonitor.configManager = &ConfigManager{}
	eventMonitor.configManager.Settings = &Settings{}
	process := &Process{
		Name:        "ProcessName",
		MonitorMode: "active",
	}
	event := Event{
		Rule:        "memory_used>2mb",
		Duration:    "10s",
		Interval:    "10s",
		Description: "The best rule ever!",
	}
	parsedEvent, _ :=
		eventMonitor.parseEvent(&event, "GroupName", "ProcessName", "stop")
	err := eventMonitor.triggerAction(process, parsedEvent, 0)
	if err != nil {
		c.Fatal(err)
	}
	c.Check(1, Equals, fc.numDoActionCalled)
	c.Check(ACTION_STOP, Equals, fc.lastActionCalled)

	parsedEvent, _ =
		eventMonitor.parseEvent(&event, "GroupName", "ProcessName", "start")
	err = eventMonitor.triggerAction(process, parsedEvent, 0)
	if err != nil {
		c.Fatal(err)
	}
	c.Check(2, Equals, fc.numDoActionCalled)
	c.Check(ACTION_START, Equals, fc.lastActionCalled)

	parsedEvent, _ =
		eventMonitor.parseEvent(&event, "GroupName", "ProcessName", "restart")
	err = eventMonitor.triggerAction(process, parsedEvent, 0)
	if err != nil {
		c.Fatal(err)
	}
	c.Check(3, Equals, fc.numDoActionCalled)
	c.Check(ACTION_RESTART, Equals, fc.lastActionCalled)

	parsedEvent, _ =
		eventMonitor.parseEvent(&event, "GroupName", "ProcessName", "alert")
	err = eventMonitor.triggerAction(process, parsedEvent, 0)
	if err != nil {
		c.Fatal(err)
	}
	c.Check(3, Equals, fc.numDoActionCalled)

	_, err =
		eventMonitor.parseEvent(&event, "GroupName", "ProcessName", "doesntexist")
	c.Check("No event action 'doesntexist' exists. Valid actions are "+
		"[stop, start, restart, alert].", Equals,
		err.Error())

	parsedEvent.action = "doesntexist"
	err = eventMonitor.triggerAction(process, parsedEvent, 0)
	c.Check("No event action 'doesntexist' exists.", Equals, err.Error())

	eventMonitor = EventMonitor{}
}

func (s *EventSuite) TestMonitoringModes(c *C) {
	fc := RegisterNewFakeControl()
	fc.isMonitoring = true
	eventMonitor.configManager = &ConfigManager{}
	eventMonitor.configManager.Settings = &Settings{}
	process := &Process{
		Name:        "ProcessName",
		MonitorMode: "active",
	}
	event := Event{
		Rule:        "memory_used>2mb",
		Duration:    "10s",
		Interval:    "10s",
		Description: "The best rule ever!",
	}
	parsedEvent, _ :=
		eventMonitor.parseEvent(&event, "GroupName", "ProcessName", "stop")
	err := eventMonitor.triggerAction(process, parsedEvent, 0)
	if err != nil {
		c.Fatal(err)
	}
	c.Check(1, Equals, fc.numDoActionCalled)
	c.Check(ACTION_STOP, Equals, fc.lastActionCalled)

	// We shoudn't trigger the action in passive mode.
	process.MonitorMode = "passive"
	parsedEvent, _ =
		eventMonitor.parseEvent(&event, "GroupName", "ProcessName", "start")
	err = eventMonitor.triggerAction(process, parsedEvent, 0)
	if err != nil {
		c.Fatal(err)
	}
	c.Check(1, Equals, fc.numDoActionCalled)
	c.Check(ACTION_STOP, Equals, fc.lastActionCalled)

	// We shouldn't trigger the action in manual mode.
	process.MonitorMode = "manual"
	parsedEvent, _ =
		eventMonitor.parseEvent(&event, "GroupName", "ProcessName", "start")
	err = eventMonitor.triggerAction(process, parsedEvent, 0)
	if err != nil {
		c.Fatal(err)
	}
	c.Check(1, Equals, fc.numDoActionCalled)
	c.Check(ACTION_STOP, Equals, fc.lastActionCalled)

	eventMonitor = EventMonitor{}
}
