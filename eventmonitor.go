// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"strings"
	"time"
)

// TODO:
// - Maybe consider changing the way rule checking works so that it timestamps
//       the last time each rule was checked instead of the way it is?
// - Debug messages?
// - Support more than just unix socket for alerts.
// - Move the parsing/validation logic from here to configmanager, since that's
//   a better fit.

// After configmanager gets the rules to be monitored, eventmonitor parses the
// rules and stores their data as ParsedEvent.
type ParsedEvent struct {
	operator     int
	ruleAmount   uint64
	resourceName string
	ruleString   string
	duration     time.Duration
	groupName    string
	processName  string
	description  string
	interval     time.Duration
	action       string
}

// The JSON message that is sent in alerts.
type AlertMessage struct {
	Action      string    `json:"action"`
	Rule        string    `json:"rule"`
	Date        time.Time `json:"date"`
	Service     string    `json:"service"`
	Description string    `json:"description"`
	Value       float64   `json:"value"`
	Message_id  uint64    `json:"message_id"`
}

const (
	DEFAULT_DURATION = "2s"
	DEFAULT_INTERVAL = "2s"
)

var validActions = []string{"stop", "start", "restart", "alert"}

const (
	EQ_OPERATOR  = 0x1
	NEQ_OPERATOR = 0x2
	GT_OPERATOR  = 0x3
	LT_OPERATOR  = 0x4
)

// Returns whether or not the actionName is a valid action.
func isValidAction(actionName string) bool {
	for _, action := range validActions {
		if actionName == action {
			return true
		}
	}
	return false
}

// Returns whether a character is an operator character in an event rule.
func isAnOperatorChar(operatorChar string) bool {
	return operatorChar == "<" || operatorChar == ">" || operatorChar == "=" ||
		operatorChar == "!"
}

// A uint64 comparison function that compares a resource's value to the
// expected value in the event rule.
func compareUint64(resourceVal uint64, operator int, ruleAmount uint64) bool {
	switch operator {
	case EQ_OPERATOR:
		return resourceVal == ruleAmount
	case NEQ_OPERATOR:
		return resourceVal != ruleAmount
	case GT_OPERATOR:
		return resourceVal > ruleAmount
	case LT_OPERATOR:
		return resourceVal < ruleAmount
	}
	return false
}

// Given a ParsedEvent and a resource value, returns whether the event rule is
// triggered.
func checkRule(parsedEvent *ParsedEvent, resourceVal uint64) bool {
	return compareUint64(resourceVal, parsedEvent.operator,
		parsedEvent.ruleAmount)
}

// Managers the monitoring of event rules.  It gets the rules from the
// configmanager, parses them, sets up a timer, then begins monitoring their
// resource values from resourcemanager.  If any events trigger, it will take
// appropriate action.
type EventMonitor struct {
	events          []*ParsedEvent
	resourceManager ResourceManager
	configManager   *ConfigManager
	control         ControlInterface
	startTime       int64
	quitChan        chan bool
}

type ControlInterface interface {
	DoAction(name string, action *ControlAction) error
	IsMonitoring(process *Process) bool
}

// Simple helper to make testing easier.
func (e *EventMonitor) registerControl(control ControlInterface) {
	e.control = control
}

// Initializes the eventmonitor by parsing event rules and initializing data
// structures.  The configmanager is where the events come from.
func (e *EventMonitor) setup(configManager *ConfigManager,
	control *Control) error {
	e.resourceManager = resourceManager
	e.configManager = configManager
	e.registerControl(control)
	e.events = []*ParsedEvent{}
	for _, group := range e.configManager.ProcessGroups {
		for _, process := range group.Processes {
			for actionName, actions := range process.Actions {
				for _, eventName := range actions {
					event := group.EventByName(eventName)
					if err := e.loadEvent(event, group.Name, process,
						actionName); err != nil {
						return fmt.Errorf("Did not load rule '%v' on action '%v' because "+
							"of error: '%v'.", eventName, actionName, err.Error())
					}
				}
			}
		}
	}
	e.startTime = time.Now().Unix()
	e.quitChan = make(chan bool)
	return nil
}

func (e *EventMonitor) printTriggeredMessage(event *ParsedEvent,
	resourceVal uint64) {
	Log.Infof("'%v' triggered '%v' for '%v' (at '%v'). Executing '%v'",
		event.processName, event.ruleString, event.duration, resourceVal,
		event.action)
}

func (e *EventMonitor) triggerAction(process *Process, event *ParsedEvent,
	resourceVal uint64) error {
	switch event.action {
	case "stop":
		if e.TriggerProcessActions(process) {
			e.printTriggeredMessage(event, resourceVal)
			return e.control.DoAction(event.processName, NewControlAction(ACTION_STOP))
		} else {
			return nil
		}
	case "start":
		if e.TriggerProcessActions(process) {
			e.printTriggeredMessage(event, resourceVal)
			return e.control.DoAction(event.processName, NewControlAction(ACTION_START))
		} else {
			return nil
		}
	case "restart":
		if e.TriggerProcessActions(process) {
			e.printTriggeredMessage(event, resourceVal)
			return e.control.DoAction(event.processName, NewControlAction(ACTION_RESTART))
		} else {
			return nil
		}
	case "alert":
		if e.TriggerAlerts(process) {
			e.printTriggeredMessage(event, resourceVal)
			return e.sendAlert(event)
		} else {
			return nil
		}
	}
	return fmt.Errorf("No event action '%v' exists.", event.action)
}

// Given a configmanager config, this function starts the eventmonitor on
// monitoring events and dispatching them.
func (e *EventMonitor) Start(configManager *ConfigManager,
	control *Control) error {
	if err := e.setup(configManager, control); err != nil {
		return err
	}
	Log.Info("Starting new eventmonitor loop.")
	go func() {
		timeToWait := 1 * time.Second
		ticker := time.NewTicker(timeToWait)
		Log.Info("Started new eventmonitor loop.")
		for {
			select {
			case <-e.quitChan:
				Log.Info("Quit old eventmonitor loop.")
				ticker.Stop()
				return
			case <-ticker.C:
				for _, group := range e.configManager.ProcessGroups {
					for _, process := range group.Processes {
						if e.IsMonitoring(process) {
							// TODO change the GetPid to be a go routine that happens every X
							// seconds with a lock on it so we don't have to keep opening the
							// file.
							pid, err := process.Pid()
							if err != nil {
								Log.Debugf("Could not get pid file for process '%v'. Error: "+
									"%+v", process.Name, err)
							}
							e.checkRules(process, pid)
						}
					}
				}
			}
		}
	}()
	return nil
}

func (e *EventMonitor) Stop() {
	Log.Info("Quitting old eventmonitor loop.")
	e.quitChan <- true
	close(e.quitChan)
	e.resourceManager.CleanData()
}

// Given a process name and a pid, this will check all the rules associated with
// it for this time period.
func (e *EventMonitor) checkRules(process *Process, pid int) {
	processName := process.Name
	diffTime := time.Now().Unix() - e.startTime
	for _, event := range e.events {
		interval := int64(event.interval.Seconds())
		if event.processName == processName &&
			(interval == 0 || diffTime%interval == 0) {
			var resourceVal uint64
			var err error
			resourceVal, err = e.resourceManager.GetResource(event, pid)
			if err != nil {
				Log.Error(err.Error())
				continue
			}
			ruleTriggered := checkRule(event, resourceVal)
			if ruleTriggered {
				// TODO right now this can block the monitoring loop.
				if err := e.triggerAction(process, event, resourceVal); err != nil {
					Log.Error(err.Error())
				}
			}
		}
	}
	e.resourceManager.ClearCachedResources()
}

// Given Events from ConfigManager, parses them and adds them to internal data
// so they can be monitored.
func (e *EventMonitor) loadEvent(event *Event, groupName string,
	process *Process, actionName string) error {
	parsedEvent, err := e.parseEvent(event, groupName, process.Name, actionName)
	if err != nil {
		return err
	}
	if err = e.validateInterval(parsedEvent); err != nil {
		return err
	}
	e.events = append(e.events, parsedEvent)
	return nil
}

// Given a rule string such as 'memory_used >= 32mb', returns the ruleAmount
// (32mb in b), operator (>=) and resourceName (memory_used).
func (e *EventMonitor) parseRule(rule string) (uint64, int, string, error) {
	startFirstPart, startLastPart := -1, -1
	var firstPart, operator, lastPart, ruleAmount, resourceName string
	operatorFound := false
	for index, theChar := range rule {
		theStr := string(theChar)
		if firstPart == "" {
			if theStr == " " && startFirstPart < 0 {
				continue
			} else if theStr != " " && startFirstPart < 0 {
				startFirstPart = index
			} else if theStr == " " || isAnOperatorChar(theStr) {
				firstPart = rule[startFirstPart:index]
			}
		}
		if isAnOperatorChar(theStr) {
			operator += theStr
			operatorFound = true
		}
		if operatorFound && !isAnOperatorChar(theStr) {
			if theStr != " " && startLastPart < 0 {
				startLastPart = index
			} else if startLastPart > 0 {
				if theStr == " " {
					lastPart = rule[startLastPart:index]
					break
				} else if index == len(rule)-1 {
					lastPart = rule[startLastPart:]
				}
			}
		}
	}
	if e.resourceManager.IsValidResourceName(firstPart) {
		ruleAmount = lastPart
		resourceName = firstPart
	} else if e.resourceManager.IsValidResourceName(lastPart) {
		ruleAmount = firstPart
		resourceName = lastPart
	} else {
		return 0, 0, "", fmt.Errorf("Using invalid resource name in rule '%v'.",
			rule)
	}
	var returnOperator int
	switch operator {
	case "==":
		returnOperator = EQ_OPERATOR
	case "!=":
		returnOperator = NEQ_OPERATOR
	case ">":
		returnOperator = GT_OPERATOR
	case "<":
		returnOperator = LT_OPERATOR
	default:
		return 0, 0, "", fmt.Errorf("Invalid operator '%v' in rule '%v'.",
			operator, rule)
	}
	parsedAmount, err := e.resourceManager.ParseAmount(resourceName, ruleAmount)
	if err != nil {
		return 0, 0, "", err
	}
	return parsedAmount, returnOperator, resourceName, nil
}

// Given an Event, parses the rule into amount, operator and resourceName, does
// a few other things, then returns a ParsedEvent ready to be monitored.
func (e *EventMonitor) parseEvent(event *Event, groupName string,
	processName string, actionName string) (*ParsedEvent, error) {
	rule := event.Rule
	ruleAmount, operator, resourceName, err := e.parseRule(rule)
	if err != nil {
		return nil, err
	}

	duration := DEFAULT_DURATION
	if event.Duration != "" {
		duration = event.Duration
	}
	parsedDuration, err := time.ParseDuration(duration)
	if err != nil {
		return nil, err
	}

	interval := DEFAULT_INTERVAL
	if event.Interval != "" {
		interval = event.Interval
	}
	parsedInterval, err := time.ParseDuration(interval)
	if err != nil {
		return nil, err
	}

	if !isValidAction(actionName) {
		return nil, fmt.Errorf("No event action '%v' exists. Valid actions "+
			"are [%+v].", actionName, strings.Join(validActions, ", "))
	}

	parsedEvent := &ParsedEvent{
		action:       actionName,
		operator:     operator,
		ruleAmount:   ruleAmount,
		resourceName: resourceName,
		ruleString:   rule,
		duration:     parsedDuration,
		groupName:    groupName,
		processName:  processName,
		description:  event.Description,
		interval:     parsedInterval,
	}
	return parsedEvent, nil
}

// Sends an alert.
func (e *EventMonitor) sendAlert(parsedEvent *ParsedEvent) error {
	settings := e.configManager.Settings
	if settings.AlertTransport == UNIX_SOCKET_TRANSPORT {
		if err := e.sendUnixSocketAlert(parsedEvent,
			settings.SocketFile); err != nil {
			return err
		}
	}
	return nil
}

func (e *EventMonitor) validateInterval(parsedEvent *ParsedEvent) error {
	for _, event := range e.events {
		if event.processName == parsedEvent.processName &&
			event.resourceName == parsedEvent.resourceName {
			if event.interval != parsedEvent.interval {
				return fmt.Errorf("Two rules ('%v' and '%v') on '%v' have different "+
					"poll intervals for the same resource '%v'.", event.ruleString,
					parsedEvent.ruleString, event.processName, event.resourceName)
			}
		}
	}
	durationRatio := parsedEvent.duration.Seconds() /
		parsedEvent.interval.Seconds()
	if parsedEvent.resourceName == CPU_PERCENT_NAME &&
		(parsedEvent.duration.Seconds()/parsedEvent.interval.Seconds()) <= 1 {
		return fmt.Errorf("Rule '%v' duration / interval must be greater "+
			"than 1.  It is '%+v / %+v'.", parsedEvent.ruleString,
			parsedEvent.duration.Seconds(), parsedEvent.interval.Seconds())
	}

	if math.Mod(durationRatio, 1.0) != 0.0 {
		return fmt.Errorf("Rule '%v' duration / interval must be an integer.",
			parsedEvent.ruleString)
	}
	return nil
}

func (e *EventMonitor) sendUnixSocketAlert(parsedEvent *ParsedEvent,
	unixSocketFile string) error {
	alertMessage := &AlertMessage{
		Action:      "alert",
		Rule:        parsedEvent.ruleString,
		Service:     parsedEvent.processName,
		Description: parsedEvent.description,
		// TODO format of date time?
		Date: time.Now(),
		// TODO implement.
		Message_id: 1234,
	}
	message, jsonError := json.Marshal(alertMessage)
	if jsonError != nil {
		return fmt.Errorf("Error marshalling json: %+v", jsonError)
	}
	c, err := net.Dial("unix", unixSocketFile)
	if err != nil {
		return fmt.Errorf("Could not connect to %v.\n", unixSocketFile)
	}
	defer c.Close()
	if _, err := c.Write(message); err != nil {
		// TODO retry logic.
		return err
	}
	return nil
}

func (e *EventMonitor) CleanDataForProcess(p *Process) {
	e.resourceManager.CleanDataForProcess(p)
}

func (e *EventMonitor) IsMonitoring(p *Process) bool {
	return e.control.IsMonitoring(p) && !p.IsMonitoringModeManual()
}

func (e *EventMonitor) StartMonitoringProcess(p *Process) {
	e.CleanDataForProcess(p)
}

func (e *EventMonitor) TriggerAlerts(p *Process) bool {
	return p.IsMonitoringModeActive() || p.IsMonitoringModePassive()
}

func (e *EventMonitor) TriggerProcessActions(p *Process) bool {
	return p.IsMonitoringModeActive()
}
