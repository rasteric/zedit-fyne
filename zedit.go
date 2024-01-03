package zedit

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
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
	builder         strings.Builder
	label           *widget.Label
	border          *fyne.Container
}

func NewZedit(columns, lines int) *Zedit {
	z := Zedit{Lines: lines, Columns: columns, grid: widget.NewTextGrid()}
	z.grid = widget.NewTextGrid()
	z.charSize = canvas.NewText("W", color.RGBA{R: 0, G: 0, B: 0, A: 255}).MinSize()
	fmt.Printf("%v\n", z.charSize)
	s := ""
	for i := 0; i < lines; i++ {
		for j := 0; j < columns; j++ {
			s += " "
		}
		s += "\n"
	}
	z.label = widget.NewLabel("")
	z.SetText(s)
	z.scroll = container.NewVScroll(z.label)
	z.scroll.OnScrolled = func(pos fyne.Position) {
		z.lineOffset = int(pos.Y / z.charSize.Height)
		z.Refresh()
	}
	z.border = container.NewBorder(nil, nil, nil, z.scroll, z.grid)
	return &z
}

func (z *Zedit) SetTopLine(x int) {
	z.lineOffset = x
	z.Refresh()
	if z.scroll != nil {
		pos := z.scroll.Offset
		z.scroll.Offset = fyne.Position{X: pos.X, Y: max(0, z.charSize.Height*float32(z.lineOffset)-float32(z.Lines))}
	}
}

func (z *Zedit) Content() fyne.CanvasObject {
	return z.border
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
	z.label.SetText(z.BuildString('\n', len(lines)))
	z.Refresh()
}

func (z *Zedit) Refresh() {
	if z.Rows != nil && len(z.Rows) > z.lineOffset {
		z.grid.Rows = z.Rows[z.lineOffset:min(z.lineOffset+z.Lines, len(z.Rows))]
	}
	z.grid.Refresh()
}

func (z *Zedit) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(z.border)
}

func (z *Zedit) BuildString(r rune, n int) string {
	z.builder.Reset()
	for i := 0; i < n; i++ {
		z.builder.WriteRune(r)
	}
	return z.builder.String()
}
