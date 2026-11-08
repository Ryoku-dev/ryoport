package common

import "image/color"

// Span is one face run in a sentence. Mono keeps the surrounding size.
// A zero-alpha Color inherits the sentence colour.
type Span struct {
	Text  string
	Mono  bool
	Color color.NRGBA
}
