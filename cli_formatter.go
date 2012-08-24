// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"
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

func (s *Summary) Print(w io.Writer) {
	writeTable(w, func(tw io.Writer) {
		for _, p := range s.Processes {
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

func (p *ProcessSummary) monitorString() string {
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

func (p *ProcessSummary) runningString() string {
	if p.Running {
		return "running"
	}
	return "not running"
}

func (p *ProcessStatus) uptime() string {
	if p.Time.StartTime == 0 {
		return "-"
	}
	start := time.Unix(int64(p.Time.StartTime)/1000, 0)
	return time.Since(start).String()
}

func (p *ProcessSummary) write(tw io.Writer) {
	fmt.Fprintf(tw, "Process '%s'\t%s\n", p.Name, p.runningString())
}

func (p *ProcessStatus) write(tw io.Writer) {
	fmt.Fprintf(tw, "Process '%s'\t\n", p.Summary.Name)

	status := []struct {
		label string
		data  interface{}
	}{
		{"status", p.Summary.runningString()},
		{"monitoring status", p.Summary.monitorString()},
		{"starts", p.Summary.ControlState.Starts},
		{"pid", p.Pid},
		{"parent pid", p.State.Ppid},
		{"uptime", p.uptime()},
		{"memory kilobytes", p.Mem.Resident / 1024},
		{"cpu", p.Time.FormatTotal()}, // TODO %cpu
		// TODO "data collected"
	}

	for _, entry := range status {
		fmt.Fprintf(tw, "  %s\t%v\n", entry.label, entry.data)
	}

	fmt.Fprintf(tw, "\t\n")
}
