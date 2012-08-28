// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"github.com/bmizerany/assert"
	"testing"
)

var eventMonitor EventMonitor

const TWO_GB = uint64(2147483648)

func init() {
	eventMonitor = EventMonitor{}
}

func TestIsAnOperatorChar(t *testing.T) {
	assert.Equal(t, true, isAnOperatorChar("="))
	assert.Equal(t, true, isAnOperatorChar(">"))
	assert.Equal(t, true, isAnOperatorChar("<"))
	assert.Equal(t, false, isAnOperatorChar("!="))
	assert.Equal(t, false, isAnOperatorChar("=="))
}

func TestCompareUint64(t *testing.T) {
	sixty := uint64(60)
	fiftyNine := uint64(59)
	assert.Equal(t, true, compareUint64(sixty, EQ_OPERATOR, 60))
	assert.Equal(t, false, compareUint64(sixty, EQ_OPERATOR, 61))

	assert.Equal(t, true, compareUint64(sixty, NEQ_OPERATOR, 50))

	assert.Equal(t, false, compareUint64(sixty, LT_OPERATOR, 50))
	assert.Equal(t, false, compareUint64(sixty, LT_OPERATOR, 60))
	assert.Equal(t, true, compareUint64(fiftyNine, LT_OPERATOR, 60))

	assert.Equal(t, true, compareUint64(sixty, GT_OPERATOR, 50))
	assert.Equal(t, false, compareUint64(sixty, GT_OPERATOR, 60))
	assert.Equal(t, false, compareUint64(fiftyNine, GT_OPERATOR, 60))
}

func TestCheckRuleUint(t *testing.T) {
	parsedEvent := &ParsedEvent{
		operator:     EQ_OPERATOR,
		ruleAmount:   uint64(7),
		resourceName: "memory_used",
	}
	resourceVal := uint64(7)
	triggering := checkRule(parsedEvent, resourceVal)
	assert.Equal(t, true, triggering)
}

func TestCheckRuleFalseUint(t *testing.T) {
	parsedEvent := &ParsedEvent{
		operator:     EQ_OPERATOR,
		ruleAmount:   uint64(8),
		resourceName: "memory_used",
	}
	resourceVal := uint64(7)
	triggering := checkRule(parsedEvent, resourceVal)
	assert.Equal(t, false, triggering)
}

func TestParseRuleForwards(t *testing.T) {
	ruleAmount, operator, resourceName, err :=
		eventMonitor.parseRule("memory_used==2gb")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, TWO_GB, ruleAmount)
	assert.Equal(t, EQ_OPERATOR, operator)
	assert.Equal(t, "memory_used", resourceName)
}

func TestParseRuleBackwards(t *testing.T) {
	ruleAmount, operator, resourceName, err :=
		eventMonitor.parseRule("2gb==memory_used")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, TWO_GB, ruleAmount)
	assert.Equal(t, EQ_OPERATOR, operator)
	assert.Equal(t, "memory_used", resourceName)
}

func TestParseRuleSpaces(t *testing.T) {
	ruleAmount, operator, resourceName, err :=
		eventMonitor.parseRule("  2gb   ==  memory_used   ")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, TWO_GB, ruleAmount)
	assert.Equal(t, EQ_OPERATOR, operator)
	assert.Equal(t, "memory_used", resourceName)
}

func TestParseRuleGt(t *testing.T) {
	ruleAmount, operator, resourceName, err :=
		eventMonitor.parseRule("2gb>memory_used")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, TWO_GB, ruleAmount)
	assert.Equal(t, GT_OPERATOR, operator)
	assert.Equal(t, "memory_used", resourceName)
}

func TestParseRuleLt(t *testing.T) {
	ruleAmount, operator, resourceName, err :=
		eventMonitor.parseRule("2gb<memory_used")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, TWO_GB, ruleAmount)
	assert.Equal(t, LT_OPERATOR, operator)
	assert.Equal(t, "memory_used", resourceName)
}

func TestParseRuleInvalidResourceError(t *testing.T) {
	_, _, _, err :=
		eventMonitor.parseRule("2gb<invalid_resource")
	assert.Equal(t, "Using invalid resource name in rule '2gb<invalid_resource'.",
		err.Error())
}

func TestParseEvent(t *testing.T) {
	event := Event{
		Rule:        "memory_used>2gb",
		Duration:    "10s",
		Interval:    "10s",
		Description: "The best rule ever!",
	}
	parsedEvent, err :=
		eventMonitor.parseEvent(&event, "GroupName", "ProcessName")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, GT_OPERATOR, parsedEvent.operator)
	assert.Equal(t, "memory_used", parsedEvent.resourceName)
	assert.Equal(t, TWO_GB, parsedEvent.ruleAmount)
}

func TestParseBadIntervalEvents(t *testing.T) {
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
	_, err := eventMonitor.parseEvent(&event1, "GroupName", "ProcessName")
	if err != nil {
		assert.Equal(t,
			"Rule 'memory_used>2gb' duration / interval must be an integer.",
			err.Error())
	}
	_, err = eventMonitor.parseEvent(&event2, "GroupName", "ProcessName")
	if err != nil {
		assert.Equal(t,
			"Rule 'cpu_percent>60' duration / interval must be greater than 1.  It "+
				"is '10 / 10'.", err.Error())
	}
}
