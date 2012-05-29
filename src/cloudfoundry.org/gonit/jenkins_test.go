package jenkinstest

import "testing"

func TestJenkins(t *testing.T) {
	if 1 != 1 {
		t.Errorf("1 isn't equal to 1!")
  }
}

func TestJenkins2(t *testing.T) {
	if 1 != 1 {
		t.Errorf("1 isn't equal to 2!")
  }
}
