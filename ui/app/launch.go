// Package app owns the window: the rail, the panes, routing, overlays,
// keyboard shortcuts and the adaptive layout.
package app

import (
	"github.com/ryoku-dev/ryoport/internal/instance"
	"github.com/ryoku-dev/ryoport/internal/model"
	"github.com/ryoku-dev/ryoport/internal/store"
)

// Launch is what main hands the window: where to open, and the messages later
// launches send to this instance.
type Launch struct {
	Select   model.ID
	Page     store.Page
	Messages <-chan instance.Message
}
