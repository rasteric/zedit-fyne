package zedit

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type Zedit struct {
	widget.BaseWidget
	Rows            []widget.TextGridRow
	Lines           int
	Columns         int
	ShowLineNumbers bool
	ShowWhitespace  bool
	TabWidth        int // If set to 0 the fyne.DefaultTabWidth is used
	grid            *widget.TextGrid
	scroll          *container.Scroll
	lineOffset      int
	charSize        fyne.Size
}

func NewZedit(columns, lines int) *Zedit {
	edit := Zedit{Lines: lines, Columns: columns, grid: widget.NewTextGrid()}
	edit.grid = widget.NewTextGrid()
	edit.scroll = container.NewScroll(edit.grid)
	edit.charSize = widget.NewLabel("W").MinSize()
	edit.scroll.OnScrolled = func(pos fyne.Position) {
		edit.lineOffset = int(pos.Y / edit.charSize.Height)
		edit.Refresh()
	}
	return &edit
}

func (z *Zedit) MinSize() fyne.Size {
	w := float32(z.Columns) * z.charSize.Width
	h := float32(len(z.Rows)) * z.charSize.Height
	return fyne.Size{Width: w, Height: h}
}

func (z *Zedit) SetTopLine(x int) {
	z.lineOffset = x
	z.Refresh()
	pos := z.scroll.Offset
	z.scroll.Offset = fyne.Position{X: pos.X, Y: float32(z.lineOffset) * z.charSize.Height}
}

func (z *Zedit) SetText(s string) {
	lines := strings.Split(s, "\n")
	z.Rows = make([]widget.TextGridRow, len(lines))
	for i, line := range lines {
		cells := make([]widget.TextGridCell, len(line))
		c := 0
		for _, char := range line {
			cells[c].Rune = char
			c++
		}
		z.Rows[i] = widget.TextGridRow{Cells: cells, Style: nil}
	}
}

func (z *Zedit) Refresh() {
	z.grid.Rows = z.Rows[z.lineOffset : z.lineOffset+z.Lines]
}

func (z *Zedit) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(z.scroll)
}
