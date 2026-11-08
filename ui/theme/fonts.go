package theme

import (
	_ "embed"
	"sync"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/text"
)

var (
	//go:embed fonts/SpaceGrotesk-Regular.otf
	spaceGroteskRegular []byte
	//go:embed fonts/SpaceGrotesk-Medium.otf
	spaceGroteskMedium []byte
	//go:embed fonts/SpaceGrotesk-Bold.otf
	spaceGroteskBold []byte
	//go:embed fonts/SpaceMono-Regular.ttf
	spaceMonoRegular []byte
	//go:embed fonts/SpaceMono-Bold.ttf
	spaceMonoBold []byte
)

var (
	collectionOnce sync.Once
	collection     []font.FontFace
)

func Collection() []font.FontFace {
	collectionOnce.Do(func() {
		faces := []struct {
			data     []byte
			typeface font.Typeface
			weight   font.Weight
		}{
			{spaceGroteskRegular, Sans, font.Normal},
			{spaceGroteskMedium, Sans, font.Medium},
			{spaceGroteskBold, Sans, font.Bold},
			{spaceMonoRegular, Mono, font.Normal},
			{spaceMonoBold, Mono, font.Bold},
		}
		for _, embedded := range faces {
			parsed, err := opentype.ParseCollection(embedded.data)
			if err != nil {
				panic(err)
			}
			for i := range parsed {
				parsed[i].Font.Typeface = embedded.typeface
				parsed[i].Font.Weight = embedded.weight
			}
			collection = append(collection, parsed...)
		}
	})
	return collection
}

func Shaper() *text.Shaper {
	return text.NewShaper(text.NoSystemFonts(), text.WithCollection(Collection()))
}
