package store

import (
	"os/exec"
	"time"

	"github.com/ryoku-dev/ryoport/internal/build"
	"github.com/ryoku-dev/ryoport/internal/console"
	"github.com/ryoku-dev/ryoport/internal/remote"
	"github.com/ryoku-dev/ryoport/internal/update"
	"github.com/ryoku-dev/ryoport/internal/vm"
	"github.com/ryoku-dev/ryoport/internal/xdg"
)

type Options struct {
	Invalidate func()
	Quit       func()
	Now        func() time.Time
	VM         *vm.Manager
	Build      *build.Builder
	Remote     *remote.Manager
	Console    *console.Manager
	Update     *update.Client
	EventsPath string
}

func normalizeOptions(o Options) Options {
	if o.Invalidate == nil {
		o.Invalidate = func() {}
	}
	if o.Quit == nil {
		o.Quit = func() {}
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.VM == nil {
		o.VM = vm.New()
	}
	if o.Build == nil {
		o.Build = build.New(o.VM)
	}
	if o.Remote == nil {
		o.Remote = remote.New()
	}
	if o.Console == nil {
		o.Console = console.New()
	}
	if o.Update == nil {
		o.Update = &update.Client{}
	}
	if o.EventsPath == "" {
		o.EventsPath = xdg.EventsFile()
	}
	return o
}

func commandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
