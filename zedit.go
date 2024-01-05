package zedit

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/chewxy/math32"
	"github.com/muesli/gamut"
)

type FixedSpacer struct {
	widget.BaseWidget
	size fyne.Size
}

func NewFixedSpacer(size fyne.Size) *FixedSpacer {
	s := FixedSpacer{size: size}
	return &s
}

func (s *FixedSpacer) Size() fyne.Size {
	return s.size
}

func (s *FixedSpacer) MinSize() fyne.Size {
	return s.size
}

func (s *FixedSpacer) ChangeSize(size fyne.Size) {
	s.size = size
}

func (s *FixedSpacer) SetHeight(height float32) {
	if s != nil {
		s.size = fyne.Size{Width: s.size.Width, Height: height}
	}
}

func (s *FixedSpacer) CreateRenderer() fyne.WidgetRenderer {
	return &FixedSpacerRenderer{s}
}

type FixedSpacerRenderer struct {
	spacer *FixedSpacer
}

func (r *FixedSpacerRenderer) Destroy() {}

func (r *FixedSpacerRenderer) Layout(size fyne.Size) {}

func (r *FixedSpacerRenderer) MinSize() fyne.Size {
	return r.spacer.size
}

func (r *FixedSpacerRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{}
}

func (r *FixedSpacerRenderer) Refresh() {}

type ZGrid struct {
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
	columnOffset    int
	charSize        fyne.Size
	border          *fyne.Container
	lineNumberStyle *widget.CustomTextGridStyle
	lineNumberGrid  *widget.TextGrid
	vSpacer         *FixedSpacer
	maxLineLen      int
	hasFocus        bool
	// focus rectangle
	focusLeft   *canvas.Line
	focusRight  *canvas.Line
	focusTop    *canvas.Line
	focusBottom *canvas.Line
	focusBorder *fyne.Container
}

func NewZGrid(columns, lines int) *ZGrid {
	bgcolor := gamut.Blends(theme.ForegroundColor(), theme.PrimaryColor(), 8)[2]
	fgcolor := theme.BackgroundColor()
	if fyne.CurrentApp().Settings().ThemeVariant() == theme.VariantDark {
		fgcolor = gamut.Tints(fgcolor, 8)[2]
		fmt.Println("DARK THEME")
	} else {
		fgcolor = gamut.Darker(fgcolor, 0.2)
	}
	z := ZGrid{Lines: lines, Columns: columns, grid: widget.NewTextGrid()}
	z.focusLeft = canvas.NewLine(theme.SelectionColor())
	z.focusRight = canvas.NewLine(theme.SelectionColor())
	z.focusTop = canvas.NewLine(theme.SelectionColor())
	z.focusBottom = canvas.NewLine(theme.SelectionColor())
	z.lineNumberStyle = &widget.CustomTextGridStyle{FGColor: fgcolor, BGColor: bgcolor}
	z.grid = widget.NewTextGrid()
	z.lineNumberGrid = widget.NewTextGrid()
	z.charSize = fyne.MeasureText("M", theme.TextSize(), fyne.TextStyle{Monospace: true})
	s := ""
	for i := 0; i < lines; i++ {
		for j := 0; j < columns; j++ {
			s += " "
		}
		s += "\n"
	}
	z.vSpacer = NewFixedSpacer(fyne.Size{Width: 0, Height: float32(z.Lines) * z.charSize.Height})

	z.scroll = container.NewVScroll(z.vSpacer)
	z.scroll.OnScrolled = func(pos fyne.Position) {
		z.lineOffset = max(0, int(math32.Round(pos.Y/z.charSize.Height)))
		z.scroll.Offset = pos
		z.Refresh()
	}
	z.SetText(s)
	z.border = container.NewBorder(nil, nil, z.lineNumberGrid, z.scroll, z.grid)
	z.focusBorder = container.NewBorder(z.focusTop, z.focusBottom, z.focusLeft, z.focusRight, z.border)
	return &z
}

func (z *ZGrid) SetLineNumberStyle(style *widget.CustomTextGridStyle) {
	z.lineNumberStyle = style
}

func (z *ZGrid) SetTopLine(x int) {
	z.lineOffset = x
	z.Refresh()
	if z.scroll != nil {
		pos := z.scroll.Offset
		z.scroll.Offset = fyne.Position{X: pos.X, Y: max(0, z.charSize.Height*float32(z.lineOffset))}
	}
}

func (z *ZGrid) ScrollRight(n int) {
	z.columnOffset = min(z.maxLineLen-z.Columns/2, z.columnOffset+n)
	z.Refresh()
}

func (z *ZGrid) ScrollLeft(n int) {
	z.columnOffset = max(0, z.columnOffset-n)
	z.Refresh()
}

func (z *ZGrid) FocusGained() {
	z.hasFocus = true
	z.Refresh()
}

func (z *ZGrid) FocusLost() {
	z.hasFocus = false
	z.Refresh()
}

func (z *ZGrid) TypedRune(rune) {

}

func (z *ZGrid) TypedKey(evt *fyne.KeyEvent) {

}

func (z *ZGrid) Content() fyne.CanvasObject {
	return z.focusBorder
}

func (z *ZGrid) SetText(s string) {
	lines := strings.Split(s, "\n")

	// populate the text grid
	z.Rows = make([]widget.TextGridRow, len(lines))
	for i, line := range lines {
		if len(line) > z.maxLineLen {
			z.maxLineLen = len(line)
		}
		cells := make([]widget.TextGridCell, len(line))
		c := 0
		for _, char := range line {
			cells[c].Rune = char
			c++
		}
		z.Rows[i] = widget.TextGridRow{Cells: cells, Style: nil}
	}

	z.vSpacer.SetHeight(float32(len(z.Rows)) * z.charSize.Height)
	z.Refresh()
}

func (z *ZGrid) TypedShortcut(s fyne.Shortcut) {
	if sc, ok := s.(*desktop.CustomShortcut); ok {
		if sc.Modifier == fyne.KeyModifierAlt {
			switch sc.KeyName {
			case fyne.KeyRight:
				z.ScrollRight(4)
			case fyne.KeyLeft:
				z.ScrollLeft(4)
			}
		}
	}
}

func (z *ZGrid) Refresh() {
	if z.hasFocus {
		z.focusLeft.Show()
		z.focusRight.Show()
		z.focusTop.Show()
		z.focusBottom.Show()
	} else {
		z.focusLeft.Hide()
		z.focusRight.Hide()
		z.focusTop.Hide()
		z.focusBottom.Hide()
	}
	if z.Rows != nil && len(z.Rows) > z.lineOffset {
		z.grid.Rows = z.Rows[z.lineOffset:min(z.lineOffset+z.Lines, len(z.Rows))]
		for i := range z.grid.Rows {
			l := len(z.grid.Rows[i].Cells)
			k := max(0, min(l-1, z.columnOffset))
			m := min(l, z.columnOffset+z.Columns)
			z.grid.Rows[i].Cells = z.grid.Rows[i].Cells[k:m]
			if l > m {
				z.grid.Rows[i].Cells[len(z.grid.Rows[i].Cells)-1].Rune = '…'
			}
			if z.columnOffset > 0 {
				z.grid.Rows[i].Cells[0].Rune = '…'
			}
		}
	}
	if z.ShowLineNumbers {
		z.lineNumberGrid.Hidden = false
		// add line numbers if necessary
		s := strconv.Itoa(len(z.Rows))
		qq := strconv.Itoa(len(s))
		fmtStr := "%" + qq + "d "
		c := z.lineOffset
		for i := 0; i < z.Lines; i++ {
			c++
			s := []rune(fmt.Sprintf(fmtStr, c))
			for j := 0; j < len(s); j++ {
				if c < len(z.Rows) {
					z.lineNumberGrid.SetCell(i, j, widget.TextGridCell{Rune: s[j], Style: z.lineNumberStyle})
				} else {
					z.lineNumberGrid.SetCell(i, j, widget.TextGridCell{Rune: ' ', Style: z.lineNumberStyle})
				}
			}
		}
		z.lineNumberGrid.Refresh()
	} else {
		z.lineNumberGrid.Hide()
	}
	z.grid.Refresh()
}

func (z *ZGrid) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(z.focusBorder)
}
