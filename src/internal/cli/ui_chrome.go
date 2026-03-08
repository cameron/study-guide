package cli

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

var paletteBlueAccentAdaptive = compat.AdaptiveColor{
	Light: color.RGBA{R: 0x78, G: 0xf0, B: 0xff, A: 0xff},
	Dark:  color.RGBA{R: 0x14, G: 0x90, B: 0xa0, A: 0xff},
}

var screenTitleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(compat.AdaptiveColor{
		Light: color.RGBA{R: 0x12, G: 0x1b, B: 0x2b, A: 0xff},
		Dark:  color.RGBA{R: 0xeb, G: 0xef, B: 0xf5, A: 0xff},
	}).
	Background(paletteBlueAccentAdaptive).
	Padding(0, 1)

func renderScreenTitle(title string) string {
	return screenTitleStyle.Render(title)
}
