// Copyright (c) 2012 VMware, Inc.

package gonit

import(
	"github.com/bmizerany/assert"
	"testing"
	"io/ioutil"
	"os"
)

// TODO more tests.  Not many now since this stuff may change.

func TestGetPid(t *testing.T) {
	file, err := ioutil.TempFile("", "pid")
	if err != nil {
		t.Error(err)
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err := file.Write([]byte("1234")); err != nil {
		t.Fatal(err)
	}
	process := Process{Pidfile: file.Name()}
	pid, err := process.GetPid()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1234, pid)
}
