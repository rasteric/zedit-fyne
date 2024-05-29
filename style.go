package zedit

import (
	"image/color"

	"fyne.io/fyne/widget"
)

type Style struct {
	FGColor, BGColor color.Color
}

func (s Style) ToTextGridStyle() widget.TextGridStyle {
	return &widget.CustomTextGridStyle{FGColor: s.FGColor, BGColor: s.BGColor}
}

type Cell struct {
	Rune  rune
	Style Style
}
