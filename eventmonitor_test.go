// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"github.com/bmizerany/assert"
	"strings"
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
	assert.Equal(t, false, isAnOperatorChar(">="))
	assert.Equal(t, false, isAnOperatorChar("<="))
	assert.Equal(t, false, isAnOperatorChar("=="))
}

func TestCompareFloat64(t *testing.T) {
	assert.Equal(t, true, compareFloat64(0.6, "==", 0.6))
	assert.Equal(t, false, compareFloat64(0.6, "==", 0.61))

	assert.Equal(t, true, compareFloat64(0.6, ">=", 0.5))
	assert.Equal(t, true, compareFloat64(0.6, ">=", 0.6))
	assert.Equal(t, false, compareFloat64(0.59, ">=", 0.6))

	assert.Equal(t, false, compareFloat64(0.6, "<=", 0.5))
	assert.Equal(t, true, compareFloat64(0.6, "<=", 0.6))
	assert.Equal(t, true, compareFloat64(0.59, "<=", 0.6))

	assert.Equal(t, false, compareFloat64(0.6, "<", 0.5))
	assert.Equal(t, false, compareFloat64(0.6, "<", 0.6))
	assert.Equal(t, true, compareFloat64(0.59, "<", 0.6))

	assert.Equal(t, true, compareFloat64(0.6, ">", 0.5))
	assert.Equal(t, false, compareFloat64(0.6, ">", 0.6))
	assert.Equal(t, false, compareFloat64(0.59, ">", 0.6))
}

func TestCompareUint64(t *testing.T) {
	assert.Equal(t, true, compareFloat64(60, "==", 60))
	assert.Equal(t, false, compareFloat64(60, "==", 61))

	assert.Equal(t, true, compareFloat64(60, ">=", 50))
	assert.Equal(t, true, compareFloat64(60, ">=", 60))
	assert.Equal(t, false, compareFloat64(59, ">=", 60))

	assert.Equal(t, false, compareFloat64(60, "<=", 50))
	assert.Equal(t, true, compareFloat64(60, "<=", 60))
	assert.Equal(t, true, compareFloat64(59, "<=", 60))

	assert.Equal(t, false, compareFloat64(60, "<", 50))
	assert.Equal(t, false, compareFloat64(60, "<", 60))
	assert.Equal(t, true, compareFloat64(59, "<", 60))

	assert.Equal(t, true, compareFloat64(60, ">", 50))
	assert.Equal(t, false, compareFloat64(60, ">", 60))
	assert.Equal(t, false, compareFloat64(59, ">", 60))
}

func TestCheckRuleFloat(t *testing.T) {
	parsedEvent := ParsedEvent{
		operator:     "==",
		ruleAmount:   0.7,
		resourceName: "mem_used",
	}
	resourceVal := 0.7
	triggering, err := checkRule(parsedEvent, resourceVal)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, true, triggering)
}

func TestCheckRuleUint(t *testing.T) {
	parsedEvent := ParsedEvent{
		operator:     "==",
		ruleAmount:   uint64(7),
		resourceName: "mem_used",
	}
	resourceVal := uint64(7)
	triggering, err := checkRule(parsedEvent, resourceVal)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, true, triggering)
}

func TestCheckRuleFalseFloat(t *testing.T) {
	parsedEvent := ParsedEvent{
		operator:     "==",
		ruleAmount:   0.8,
		resourceName: "mem_used",
	}
	resourceVal := 0.7
	triggering, err := checkRule(parsedEvent, resourceVal)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, false, triggering)
}

func TestCheckRuleFalseUint(t *testing.T) {
	parsedEvent := ParsedEvent{
		operator:     "==",
		ruleAmount:   uint64(8),
		resourceName: "mem_used",
	}
	resourceVal := uint64(7)
	triggering, err := checkRule(parsedEvent, resourceVal)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, false, triggering)
}

func TestCheckRuleError(t *testing.T) {
	parsedEvent := ParsedEvent{
		operator:     "==",
		ruleAmount:   true,
		resourceName: "mem_used",
	}
	resourceVal := true
	_, err := checkRule(parsedEvent, resourceVal)
	assert.Equal(t, "Resource 'mem_used' with value 'true' is not a known type.",
		err.Error())
}

func TestParseRuleForwards(t *testing.T) {
	ruleAmount, operator, resourceName, err :=
		eventMonitor.parseRule("mem_used==2gb")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, TWO_GB, ruleAmount.(uint64))
	assert.Equal(t, "==", operator)
	assert.Equal(t, "mem_used", resourceName)
}

func TestParseRuleBackwards(t *testing.T) {
	ruleAmount, operator, resourceName, err :=
		eventMonitor.parseRule("2gb==mem_used")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, TWO_GB, ruleAmount.(uint64))
	assert.Equal(t, "==", operator)
	assert.Equal(t, "mem_used", resourceName)
}

func TestParseRuleSpaces(t *testing.T) {
	ruleAmount, operator, resourceName, err :=
		eventMonitor.parseRule("  2gb   ==  mem_used   ")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, TWO_GB, ruleAmount.(uint64))
	assert.Equal(t, "==", operator)
	assert.Equal(t, "mem_used", resourceName)
}

func TestParseRuleGt(t *testing.T) {
	ruleAmount, operator, resourceName, err :=
		eventMonitor.parseRule("2gb>mem_used")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, TWO_GB, ruleAmount.(uint64))
	assert.Equal(t, ">", operator)
	assert.Equal(t, "mem_used", resourceName)
}

func TestParseRuleLt(t *testing.T) {
	ruleAmount, operator, resourceName, err :=
		eventMonitor.parseRule("2gb<mem_used")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, TWO_GB, ruleAmount.(uint64))
	assert.Equal(t, "<", operator)
	assert.Equal(t, "mem_used", resourceName)
}

func TestParseRuleInvalidResourceError(t *testing.T) {
	_, _, _, err :=
		eventMonitor.parseRule("2gb<invalid_resource")
	assert.Equal(t, true, strings.HasPrefix(err.Error(),
		"Invalid resource name in rule '2gb<invalid_resource'.  Valid resources "+
			"are"))
}

func TestParseEvent(t *testing.T) {
	event := Event{
		Rule:        "mem_used>2gb",
		Duration:    "8s",
		Interval:    "10s",
		Description: "The best rule ever!",
	}
	parsedEvent, err :=
		eventMonitor.parseEvent(event, "GroupName", 1234, "ProcessName")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, ">", parsedEvent.operator)
	assert.Equal(t, "mem_used", parsedEvent.resourceName)
	assert.Equal(t, TWO_GB, parsedEvent.ruleAmount)
}
