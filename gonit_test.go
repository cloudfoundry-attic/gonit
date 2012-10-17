// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	. "launchpad.net/gocheck"
	"regexp"
)

type GonitSuite struct{}

var _ = Suite(&GonitSuite{})

func (s *GonitSuite) TestVersion(c *C) {
	pattern := regexp.MustCompile(`^\d+\.\d+\.\d+$`)

	if !pattern.MatchString(VERSION) {
		c.Errorf("Invalid VERSION=%s", VERSION)
	}
}
