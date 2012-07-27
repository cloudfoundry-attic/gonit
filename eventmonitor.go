// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"
)

// TODO:
// - Support more actions than alert.
// - Maybe consider changing the way rule checking works so that it timestamps
//       the last time each rule was checked instead of the way it is?
// - Debug messages?

// After configmanager gets the rules to be monitored, eventmonitor parses the
// rules and stores their data as ParsedEvent.
type ParsedEvent struct {
	operator     int
	ruleAmount   interface{}
	resourceName string
	ruleString   string
	duration     time.Duration
	groupName    string
	processName  string
	description  string
	interval     time.Duration
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
	DEFAULT_DURATION = "0s"
	DEFAULT_INTERVAL = "2s"
)

const (
	EQ_OPERATOR  = 0x1
	NEQ_OPERATOR = 0x2
	GT_OPERATOR  = 0x3
	LT_OPERATOR  = 0x4
)

// Returns whether a character is an operator character in an event rule.
func isAnOperatorChar(operatorChar string) bool {
	return operatorChar == "<" || operatorChar == ">" || operatorChar == "=" ||
		operatorChar == "!"
}

// A float64 comparison function that compares a resource's value to the
// expected value in the event rule.
func compareFloat64(resourceVal float64, operator int,
	ruleAmount float64) bool {
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
func checkRule(parsedEvent *ParsedEvent, resourceVal interface{}) (bool,
	error) {
	switch resourceVal.(type) {
	case float64:
		rv := resourceVal.(float64)
		return compareFloat64(rv, parsedEvent.operator,
			parsedEvent.ruleAmount.(float64)), nil
	case uint64:
		rv := resourceVal.(uint64)
		return compareUint64(rv, parsedEvent.operator,
			parsedEvent.ruleAmount.(uint64)), nil
	}
	return false, fmt.Errorf("Resource '%v' with value '%v' is not a known type.",
		parsedEvent.resourceName, resourceVal)
}

// Managers the monitoring of event rules.  It gets the rules from the
// configmanager, parses them, sets up a timer, then begins monitoring their
// resource values from resourcemanager.  If any events trigger, it will take
// appropriate action.
type EventMonitor struct {
	alertEvents     []*ParsedEvent
	resourceManager ResourceManager
	unixSocketFile  string
	configManager   *ConfigManager
	startTime       int64
	quitChan        chan bool
}

// Initializes the eventmonitor by parsing event rules and initializing data
// structures.  The configmanager is where the events come from, and the
// unixSocketFile is where alerts are sent to.
func (e *EventMonitor) setup(configManager *ConfigManager,
	unixSocketFile string) error {
	e.unixSocketFile = unixSocketFile
	e.resourceManager = resourceManager
	e.configManager = configManager
	e.alertEvents = []*ParsedEvent{}
	for _, group := range e.configManager.ProcessGroups {
		for _, process := range group.Processes {
			for _, actionNames := range process.Actions {
				for _, actionName := range actionNames {
					err := e.loadEvents(group.EventsFromAction(actionName), group.Name,
						process)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	e.startTime = time.Now().Unix()
	e.quitChan = make(chan bool)
	return nil
}

// Given a configmanager config and a unix socket to send alerts to, this
// function starts the eventmonitor on monitoring events and dispatching them.
func (e *EventMonitor) Start(configManager *ConfigManager,
	unixSocketFile string) error {
	err := e.setup(configManager, unixSocketFile)
	if err != nil {
		return err
	}

	go func() {
		timeToWait := 1 * time.Second
		ticker := time.NewTicker(timeToWait)
		log.Println("Starting eventmonitor loop.")
		for {
			select {
			case <-e.quitChan:
				log.Println("Quitting eventmonitor loop.")
				ticker.Stop()
				return
			case <-ticker.C:
				for _, group := range e.configManager.ProcessGroups {
					for _, process := range group.Processes {
						// TODO change the GetPid to be a go routine that happens every X
						// seconds with a lock on it so we don't have to keep opening the
						// file.
						pid, err := process.GetPid()
						if err != nil {
							log.Println("Could not get pid file for process '%v'. Error: %+v",
								process.Name, err)
						}
						if process.Monitor != false {
							e.checkRules(process.Name, pid)
						}
					}
				}
			}
		}
	}()
	return nil
}

func (e *EventMonitor) Stop() {
	// TODO: Clean up data structures.
	e.quitChan <- true
	close(e.quitChan)
	e.resourceManager.CleanData()
}

// Given a process name and a pid, this will check all the rules associated with
// it for this time period.
func (e *EventMonitor) checkRules(processName string, pid int) {
	diffTime := time.Now().Unix() - e.startTime
	for _, alertEvent := range e.alertEvents {
		interval := int64(alertEvent.interval.Seconds())
		if alertEvent.processName == processName &&
			(interval == 0 || diffTime%interval == 0) {
			resourceVal, err := e.resourceManager.GetResource(alertEvent, pid)
			if err != nil {
				log.Println(err)
				continue
			}
			ruleTriggered, err := checkRule(alertEvent, resourceVal)
			if err != nil {
				log.Println(err)
				continue
			}
			if ruleTriggered {
				// TODO right now this can block the monitoring loop.
				e.sendAlert(alertEvent)
			}
		}
	}
}

// Given Events from ConfigManager, parses them and adds them to internal data
// so they can be monitored.
func (e *EventMonitor) loadEvents(events []Event, groupName string,
	process Process) error {
	for _, event := range events {
		parsedEvent, err := e.parseEvent(event, groupName, process.Name)
		if err != nil {
			return err
		}
		e.alertEvents = append(e.alertEvents, parsedEvent)
	}
	return nil
}

// Given a rule string such as 'memory_used >= 32mb', returns the ruleAmount
// (32mb in b), operator (>=) and resourceName (memory_used).
func (e *EventMonitor) parseRule(
	rule string) (interface{}, int, string, error) {
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
func (e *EventMonitor) parseEvent(event Event, groupName string,
	processName string) (*ParsedEvent, error) {
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

	parsedEvent := &ParsedEvent{
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

// Sends an alert over the unix socket.
func (e *EventMonitor) sendAlert(parsedEvent *ParsedEvent) error {
	// TODO let this send more ways than just over unix socket.  Also, have
	// configuration in the config file.
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
	log.Printf("Rule '%v' for process '%v' has triggered for > %v seconds.\n",
		parsedEvent.ruleString, parsedEvent.processName, parsedEvent.duration)
	c, err := net.Dial("unix", e.unixSocketFile)
	if err != nil {
		return fmt.Errorf("Could not connect to %v.\n", e.unixSocketFile)
	}
	defer c.Close()
	if _, err := c.Write(message); err != nil {
		// TODO retry logic.
		return err
	}
	return nil
}
