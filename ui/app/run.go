package app

import (
	"context"

	gioapp "gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/x/explorer"

	internalpalette "github.com/ryoku-dev/ryoport/internal/palette"
	"github.com/ryoku-dev/ryoport/internal/store"
	"github.com/ryoku-dev/ryoport/ui/theme"
)

const applicationID = "dev.ryoku.ryoport"

func init() {
	gioapp.ID = applicationID
}

// Run owns the window until the platform destroys it.
func Run(l Launch) error {
	var window gioapp.Window
	window.Option(
		gioapp.Title("Ryoport"),
		gioapp.Size(unit.Dp(1280), unit.Dp(800)),
		gioapp.MinSize(unit.Dp(640), unit.Dp(520)),
		gioapp.Decorated(false),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	posts := newPostQueue(window.Invalidate)
	state := store.New(store.Options{
		Invalidate: window.Invalidate,
		Quit:       func() { window.Perform(system.ActionClose) },
	})
	state.Run(ctx)

	initialSnapshot := state.Snapshot()
	initialPalette := internalpalette.Load(paletteMode(initialSnapshot))
	sh := newShell(&window, state, posts.Post, theme.New(theme.FromPalette(initialPalette), theme.Shaper()))
	if initialSnapshot != nil {
		sh.appearanceMode = initialSnapshot.Settings.Appearance.Mode
	}
	sh.route(l.Select, l.Page)

	fileExplorer := explorer.NewExplorer(&window)
	sh.files = newFileChooser(fileExplorer, posts.Post)

	go func() {
		_ = internalpalette.Watch(ctx, func() internalpalette.Mode {
			return paletteMode(state.Snapshot())
		}, func(p internalpalette.Palette) {
			posts.Post(func() { sh.setPalette(p) })
		})
	}()
	sh.listen(ctx, l.Messages, posts.Post)

	var ops op.Ops
	for {
		event := window.Event()
		fileExplorer.ListenEvents(event)
		switch event := event.(type) {
		case gioapp.ConfigEvent:
			sh.Config(event.Config)
		case gioapp.FrameEvent:
			posts.Drain()
			sh.Frame(event, &ops, state.Snapshot())
			event.Frame(&ops)
		case gioapp.DestroyEvent:
			cancel()
			return nil
		}
	}
}

func paletteMode(snapshot *store.Snapshot) internalpalette.Mode {
	if snapshot == nil {
		return internalpalette.Follow
	}
	return paletteModeValue(snapshot.Settings.Appearance.Mode)
}

func paletteModeValue(mode string) internalpalette.Mode {
	switch mode {
	case "dark":
		return internalpalette.Dark
	case "light":
		return internalpalette.Light
	default:
		return internalpalette.Follow
	}
}
