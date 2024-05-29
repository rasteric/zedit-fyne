package zedit

import (
	"image/color"

	"fyne.io/fyne/v2/widget"
)

type Style struct {
	FGColor, BGColor color.Color
}

var EmptyStyle = Style{}

func (s Style) ToTextGridStyle() widget.TextGridStyle {
	return &widget.CustomTextGridStyle{FGColor: s.FGColor, BGColor: s.BGColor}
}

type Cell struct {
	Rune  rune
	Style Style
}

func (c Cell) ToTextGridCell() widget.TextGridCell {
	return widget.TextGridCell{Rune: c.Rune, Style: c.Style.ToTextGridStyle()}
}

func NewCellFromTextGridCell(cell widget.TextGridCell) Cell {
	if cell.Style == nil {
		return Cell{Rune: cell.Rune, Style: Style{}}
	}
	return Cell{
		Rune:  cell.Rune,
		Style: Style{FGColor: cell.Style.TextColor(), BGColor: cell.Style.BackgroundColor()},
	}
}
