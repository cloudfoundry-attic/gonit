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
	assert.Equal(t, false, isAnOperatorChar("!="))
	assert.Equal(t, false, isAnOperatorChar("=="))
}

func TestCompareFloat64(t *testing.T) {
	pointSix := float64(0.6)
	pointFiveNine := float64(0.59)
	assert.Equal(t, true, compareFloat64(&pointSix, EQ_OPERATOR, 0.6))
	assert.Equal(t, false, compareFloat64(&pointSix, EQ_OPERATOR, 0.61))

	assert.Equal(t, true, compareFloat64(&pointSix, NEQ_OPERATOR, 0.5))

	assert.Equal(t, false, compareFloat64(&pointSix, LT_OPERATOR, 0.5))
	assert.Equal(t, false, compareFloat64(&pointSix, LT_OPERATOR, 0.6))
	assert.Equal(t, true, compareFloat64(&pointFiveNine, LT_OPERATOR, 0.6))

	assert.Equal(t, true, compareFloat64(&pointSix, GT_OPERATOR, 0.5))
	assert.Equal(t, false, compareFloat64(&pointSix, GT_OPERATOR, 0.6))
	assert.Equal(t, false, compareFloat64(&pointFiveNine, GT_OPERATOR, 0.6))
}

func TestCompareUint64(t *testing.T) {
	sixty := uint64(60)
	fiftyNine := uint64(59)
	assert.Equal(t, true, compareUint64(&sixty, EQ_OPERATOR, 60))
	assert.Equal(t, false, compareUint64(&sixty, EQ_OPERATOR, 61))

	assert.Equal(t, true, compareUint64(&sixty, NEQ_OPERATOR, 50))

	assert.Equal(t, false, compareUint64(&sixty, LT_OPERATOR, 50))
	assert.Equal(t, false, compareUint64(&sixty, LT_OPERATOR, 60))
	assert.Equal(t, true, compareUint64(&fiftyNine, LT_OPERATOR, 60))

	assert.Equal(t, true, compareUint64(&sixty, GT_OPERATOR, 50))
	assert.Equal(t, false, compareUint64(&sixty, GT_OPERATOR, 60))
	assert.Equal(t, false, compareUint64(&fiftyNine, GT_OPERATOR, 60))
}

func TestCheckRuleFloat(t *testing.T) {
	parsedEvent := ParsedEvent{
		operator:     EQ_OPERATOR,
		ruleAmount:   0.7,
		resourceName: "memory_used",
	}
	resourceVal := 0.7
	triggering, err := checkRule(&parsedEvent, resourceVal)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, true, triggering)
}

func TestCheckRuleUint(t *testing.T) {
	parsedEvent := ParsedEvent{
		operator:     EQ_OPERATOR,
		ruleAmount:   uint64(7),
		resourceName: "memory_used",
	}
	resourceVal := uint64(7)
	triggering, err := checkRule(&parsedEvent, resourceVal)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, true, triggering)
}

func TestCheckRuleFalseFloat(t *testing.T) {
	parsedEvent := ParsedEvent{
		operator:     EQ_OPERATOR,
		ruleAmount:   0.8,
		resourceName: "memory_used",
	}
	resourceVal := 0.7
	triggering, err := checkRule(&parsedEvent, resourceVal)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, false, triggering)
}

func TestCheckRuleFalseUint(t *testing.T) {
	parsedEvent := ParsedEvent{
		operator:     EQ_OPERATOR,
		ruleAmount:   uint64(8),
		resourceName: "memory_used",
	}
	resourceVal := uint64(7)
	triggering, err := checkRule(&parsedEvent, resourceVal)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, false, triggering)
}

func TestCheckRuleError(t *testing.T) {
	parsedEvent := ParsedEvent{
		operator:     EQ_OPERATOR,
		ruleAmount:   true,
		resourceName: "memory_used",
	}
	resourceVal := true
	_, err := checkRule(&parsedEvent, resourceVal)
	assert.Equal(t, "Resource 'memory_used' with value 'true' is not a known "+
		"type.", err.Error())
}

func TestParseRuleForwards(t *testing.T) {
	parsedEvent, err := eventMonitor.parseRule("memory_used==2gb")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, TWO_GB, parsedEvent.ruleAmount.(uint64))
	assert.Equal(t, EQ_OPERATOR, parsedEvent.operator)
	assert.Equal(t, "memory_used", parsedEvent.resourceName)
}

func TestParseRuleBackwards(t *testing.T) {
	parsedEvent, err := eventMonitor.parseRule("2gb==memory_used")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, TWO_GB, parsedEvent.ruleAmount.(uint64))
	assert.Equal(t, EQ_OPERATOR, parsedEvent.operator)
	assert.Equal(t, "memory_used", parsedEvent.resourceName)
}

func TestParseRuleSpaces(t *testing.T) {
	parsedEvent, err := eventMonitor.parseRule("  2gb   ==  memory_used   ")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, TWO_GB, parsedEvent.ruleAmount.(uint64))
	assert.Equal(t, EQ_OPERATOR, parsedEvent.operator)
	assert.Equal(t, "memory_used", parsedEvent.resourceName)
}

func TestParseRuleGt(t *testing.T) {
	parsedEvent, err := eventMonitor.parseRule("2gb>memory_used")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, TWO_GB, parsedEvent.ruleAmount.(uint64))
	assert.Equal(t, GT_OPERATOR, parsedEvent.operator)
	assert.Equal(t, "memory_used", parsedEvent.resourceName)
}

func TestParseRuleLt(t *testing.T) {
	parsedEvent, err := eventMonitor.parseRule("2gb<memory_used")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, TWO_GB, parsedEvent.ruleAmount.(uint64))
	assert.Equal(t, LT_OPERATOR, parsedEvent.operator)
	assert.Equal(t, "memory_used", parsedEvent.resourceName)
}

func TestParseRuleInvalidResourceError(t *testing.T) {
	_, err := eventMonitor.parseRule("2gb<invalid_resource")
	assert.Equal(t, true, strings.HasPrefix(err.Error(),
		"Invalid resource name in rule '2gb<invalid_resource'.  Valid resources "+
			"are"))
}

func TestParseEvent(t *testing.T) {
	event := Event{
		Rule:        "memory_used>2gb",
		Duration:    "8s",
		Interval:    "10s",
		Description: "The best rule ever!",
	}
	parsedEvent, err :=
		eventMonitor.parseEvent(event, "GroupName", "ProcessName")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, GT_OPERATOR, parsedEvent.operator)
	assert.Equal(t, "memory_used", parsedEvent.resourceName)
	assert.Equal(t, TWO_GB, parsedEvent.ruleAmount)
}
