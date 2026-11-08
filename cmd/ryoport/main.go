package main

import (
	"context"
	"fmt"
	"os"

	gioapp "gioui.org/app"

	"github.com/ryoku-dev/ryoport/internal/cli"
	"github.com/ryoku-dev/ryoport/internal/instance"
	app "github.com/ryoku-dev/ryoport/ui/app"
)

func main() {
	code, gui := cli.Main(os.Args, os.Stdout, os.Stderr)
	if !gui {
		os.Exit(code)
	}
	token := os.Getenv("XDG_ACTIVATION_TOKEN")
	if token == "" {
		token = os.Getenv("DESKTOP_STARTUP_ID")
	}
	message := instance.Message{Args: os.Args[1:], Token: token}
	if sent, err := instance.Send(message); err != nil {
		fmt.Fprintln(os.Stderr, "ryoport:", err)
		os.Exit(1)
	} else if sent {
		os.Exit(0)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	messages, err := instance.Listen(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ryoport:", err)
		os.Exit(1)
	}
	selected, page := cli.ParseWindow(os.Args[1:])
	go func() {
		err := app.Run(app.Launch{Select: selected, Page: page, Messages: messages})
		if err != nil {
			fmt.Fprintln(os.Stderr, "ryoport:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()
	gioapp.Main()
}
