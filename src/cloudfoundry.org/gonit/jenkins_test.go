package jenkinstest

import "testing"

func TestJenkins(t *testing.T) {
	if 1 != 3 {
		t.Errorf("1 isn't equal to 1!")
  }
}
