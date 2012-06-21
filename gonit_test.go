// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"regexp"
	"testing"
)

func TestVersion(t *testing.T) {
	pattern := regexp.MustCompile(`^\d+\.\d+\.\d+$`)

	if !pattern.MatchString(VERSION) {
		t.Errorf("Invalid VERSION=%s", VERSION)
	}
}
