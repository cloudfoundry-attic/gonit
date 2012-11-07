// Copyright (c) 2012 VMware, Inc.

package gonit

import (
	"github.com/cloudfoundry/gosigar/psnotify"
	"time"
)

type Watcher struct {
	Control *Control
	quit    chan bool
	notify  *psnotify.Watcher
	pids    map[int]*Process
}

func (w *Watcher) checkProcess(process *Process) {
	state := w.Control.State(process)

	if state.Monitor == MONITOR_NOT {
		Log.Debugf("Process %q is not monitored", process.Name)
		return
	}

	state.Monitor = MONITOR_YES

	Log.Debugf("Checking process %q", process.Name)

	if !process.IsRunning() {
		// TODO: alert
		w.Control.DoAction(process.Name, ACTION_START)
	}

	if w.usingNotify() {
		if pid, err := process.Pid(); err == nil {
			if _, exists := w.pids[pid]; !exists {
				w.notify.Watch(pid, psnotify.PROC_EVENT_EXIT)
				w.pids[pid] = process
			}
		}
	}
}

func (w *Watcher) Check() {
	for _, group := range w.Control.Config().ProcessGroups {
		for _, process := range group.Processes {
			w.checkProcess(process)
		}
	}
}

func (w *Watcher) run() {
	w.Check()

	config := w.Control.Config()
	interval := time.Duration(config.Settings.PollInterval) * time.Second
	ticker := time.NewTicker(interval)

	Log.Info("Starting process watcher")
	var exits chan *psnotify.ProcEventExit

	if w.usingNotify() {
		exits = w.notify.Exit
	}

	for {
		select {
		case <-w.quit:
			Log.Info("Quit process watcher")
			ticker.Stop()
			return
		case ev := <-exits:
			if process, exists := w.pids[ev.Pid]; exists {
				Log.Infof("Process %q exit, pid=%d",
					process.Name, ev.Pid)
				delete(w.pids, ev.Pid)
				w.checkProcess(process)
			}
		case <-ticker.C:
			w.Check()
		}
	}
}

func (w *Watcher) Start() {
	var err error
	w.notify, err = psnotify.NewWatcher()
	if err == nil {
		Log.Info("psnotify: enabled")
		w.pids = make(map[int]*Process)
	} else {
		Log.Warnf("psnotify disabled: %s", err)
	}

	w.quit = make(chan bool)

	go w.run()
}

func (w *Watcher) Stop() {
	Log.Info("Stopping process watcher")

	if w.usingNotify() {
		w.notify.Close()
	}

	w.quit <- true
	close(w.quit)
}

func (w *Watcher) usingNotify() bool {
	return w.notify != nil
}
