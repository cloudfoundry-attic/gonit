package gonit

import (
	"github.com/cloudfoundry/gosteno"
	"os"
)

var Log steno.Logger

func init() {
	Init(nil)
}

func Init(settings *Settings) {
	var out steno.Sink
	var logfile string

	if settings != nil {
		if settings.Daemon.Stdout != "" {
			logfile = settings.Daemon.Stdout
		}
	}

	if logfile == "" {
		out = steno.NewIOSink(os.Stdout)
	} else {
		out = steno.NewFileSink(logfile)
	}

	cfg := &steno.Config{
		EnableLOC: true,
		Sinks:     []steno.Sink{out},
		// TODO Level:
	}

	steno.Init(cfg)

	out.SetCodec(steno.NewJsonPrettifier(steno.EXCLUDE_DATA))

	Log = steno.NewLogger("gonit")
}
