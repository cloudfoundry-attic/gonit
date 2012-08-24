// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"fmt"
	"io"
	"text/tabwriter"
)

// format data returned from the API
type CliFormatter interface {
	Print(w io.Writer)
}

func (p *ProcessStatus) Print(w io.Writer) {
	writeTable(w, func(tw io.Writer) {
		p.write(tw)
	})
}

func (g *ProcessGroupStatus) Print(w io.Writer) {
	writeTable(w, func(tw io.Writer) {
		for _, p := range g.Group {
			p.write(tw)
		}
	})
}

func writeTable(w io.Writer, f func(io.Writer)) {
	tw := new(tabwriter.Writer)
	tw.Init(w, 0, 8, 8, ' ', 0)
	f(tw)
	tw.Flush()
}

func (p *ProcessStatus) monitorString() string {
	monitor := []struct {
		mode  int
		label string
	}{
		{MONITOR_WAITING, "waiting"},
		{MONITOR_INIT, "initializing"},
		{MONITOR_YES, "monitored"},
		{MONITOR_NOT, "not monitored"},
	}

	for _, state := range monitor {
		if (p.ControlState.Monitor & state.mode) == state.mode {
			return state.label
		}
	}

	panic("not reached")
}

func (p *ProcessStatus) runningString() string {
	if p.Running {
		return "running"
	}
	return "not running"
}

func (p *ProcessStatus) write(tw io.Writer) {
	fmt.Fprintf(tw, "Process '%s'\t\n", p.Name)

	status := []struct {
		label string
		data  interface{}
	}{
		{"status", p.runningString()},
		{"monitoring status", p.monitorString()},
		{"starts", p.ControlState.Starts},
		{"pid", p.Pid},
		{"parent pid", p.State.Ppid},
		{"uptime", p.Time.FormatStartTime()},
		{"memory kilobytes", p.Mem.Resident / 1024},
		// XXX "cpu", "data collected"
	}

	for _, entry := range status {
		fmt.Fprintf(tw, "  %s\t%v\n", entry.label, entry.data)
	}

	fmt.Fprintf(tw, "\t\n")
}
