package zedit

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
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
	Tags            *TagContainer
	Lines           int
	Columns         int
	ShowLineNumbers bool
	ShowWhitespace  bool
	ScrollFactor    float32
	TabWidth        int // If set to 0 the fyne.DefaultTabWidth is used
	grid            *widget.TextGrid
	scroll          *container.Scroll
	lineOffset      int
	columnOffset    int
	charSize        fyne.Size
	border          *fyne.Container
	defaultStyle    *widget.CustomTextGridStyle
	lineNumberStyle *widget.CustomTextGridStyle
	lineNumberGrid  *widget.TextGrid
	vSpacer         *FixedSpacer
	maxLineLen      int
	hasFocus        bool
	background      *canvas.Rectangle
	content         *fyne.Container
	selStart        *CharPos
	selEnd          *CharPos
}

// ZGrid is like TextGrid but with an internal vertical scroll bar and an optional line number display.
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
	z.Tags = NewTagContainer()
	z.ScrollFactor = 2.0
	z.defaultStyle = &widget.CustomTextGridStyle{FGColor: theme.ForegroundColor(), BGColor: theme.InputBackgroundColor()}
	z.lineNumberStyle = &widget.CustomTextGridStyle{FGColor: fgcolor, BGColor: bgcolor}
	z.grid = widget.NewTextGrid()
	z.background = canvas.NewRectangle(theme.InputBackgroundColor()) //theme.InputBackgroundColor())
	z.background.StrokeColor = theme.FocusColor()
	z.background.StrokeWidth = 4
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

	z.scroll = container.NewScroll(z.vSpacer)
	z.scroll.OnScrolled = func(pos fyne.Position) {
		z.lineOffset = max(0, int(math32.Round(pos.Y/z.charSize.Height)))
		z.scroll.Offset = pos
		z.hasFocus = true
		z.Refresh()
	}
	z.SetText(s)
	z.border = container.NewBorder(nil, nil, z.lineNumberGrid, z.scroll, z.grid)
	z.content = container.New(layout.NewStackLayout(), z.background, z.border)
	z.Tags.AddStyler(TagStyler{Tag: SimpleTag{"selection"}, StyleFunc: z.SelectionStyleFunc()})
	return &z
}

func (z *ZGrid) SetLineNumberStyle(style *widget.CustomTextGridStyle) {
	z.lineNumberStyle = style
}

func (z *ZGrid) SelectionStyleFunc() TagStyleFunc {
	return TagStyleFunc(func(c widget.TextGridCell) widget.TextGridCell {
		selStyle := &widget.CustomTextGridStyle{FGColor: theme.ForegroundColor(), BGColor: theme.SelectionColor()}
		return widget.TextGridCell{
			Rune:  c.Rune,
			Style: selStyle,
		}
	})
}

// SetTopLine sets the zgrid to display starting with the given line number.
func (z *ZGrid) SetTopLine(x int) {
	z.lineOffset = x
	z.Refresh()
	if z.scroll != nil {
		pos := z.scroll.Offset
		z.scroll.Offset = fyne.Position{X: pos.X, Y: max(0, z.charSize.Height*float32(z.lineOffset))}
	}
}

// Row returns the row at line i. The row returned is not a copy but the original.
func (z *ZGrid) Row(i int) widget.TextGridRow {
	if i < 0 || i >= len(z.Rows) {
		return widget.TextGridRow{}
	}
	return z.Rows[i]
}

func (z *ZGrid) ScrollDown() {
	li := min(len(z.Rows)-z.Lines/2, z.lineOffset+1)
	z.SetTopLine(li)
}

func (z *ZGrid) ScrollUp() {
	li := max(0, z.lineOffset-1)
	z.SetTopLine(li)
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
	z.background.StrokeColor = theme.FocusColor()
	z.background.Refresh()
	z.Refresh()
}

func (z *ZGrid) FocusLost() {
	z.hasFocus = false
	z.background.StrokeColor = theme.BackgroundColor()
	z.background.Refresh()
	z.Refresh()
}

func (z *ZGrid) MouseIn(evt *desktop.MouseEvent) {}

func (z *ZGrid) MouseMoved(evt *desktop.MouseEvent) {}

func (z *ZGrid) MouseOut() {}

func (z *ZGrid) Scrolled(evt *fyne.ScrollEvent) {
	step := z.ScrollFactor * (evt.Scrolled.DY / z.charSize.Height)
	z.lineOffset = min(len(z.Rows)-z.Lines/2, max(0, int(float32(z.lineOffset)-step)))
	z.scroll.Offset = fyne.Position{X: z.scroll.Offset.X, Y: float32(z.lineOffset) * z.charSize.Height}
	z.scroll.Refresh()
	z.Refresh()
}

func (z *ZGrid) Dragged(evt *fyne.DragEvent) {
	pos := z.PosToCharPos(evt.Position)
	if z.selStart == nil {
		z.selStart = &pos
		return
	}
	z.selEnd = &pos
	tag := SimpleTag{"selection"}
	interval := CharInterval{Start: *z.selStart, End: *z.selEnd}.MaybeSwap()
	z.Tags.Upsert(tag, interval)
	if pos.Line <= z.lineOffset {
		z.ScrollUp()
	} else if pos.Line >= z.lineOffset+z.Lines-1 {
		z.ScrollDown()
	}
	z.Refresh()
	fmt.Printf("selection start=%v end=%v\n", interval.Start, interval.End)
}

func (z *ZGrid) Cursor() desktop.Cursor {
	return desktop.TextCursor
}

func (z *ZGrid) Tapped(evt *fyne.PointEvent) {
	z.RemoveSelection()
}

func (z *ZGrid) DragEnd() {
	z.selStart = nil
	z.selEnd = nil
}

func (z *ZGrid) TypedRune(rune) {}

func (z *ZGrid) TypedKey(evt *fyne.KeyEvent) {}

// SELECTION HANDLING

// RemoveSelection removes the current selection, both the range returned by GetSelection
// and its graphical display.
func (z *ZGrid) RemoveSelection() {
	z.Tags.Delete(SimpleTag{"selection"})
	z.selStart = nil
	z.selEnd = nil
	z.Refresh()
}

// PosToCharPos converts an internal position of the widget in Fyne's pixel unit to a
// line, row pair.
func (z *ZGrid) PosToCharPos(pos fyne.Position) CharPos {
	x := pos.X - z.lineNumberGrid.Size().Width
	y := pos.Y
	if z.lineNumberGrid.Visible() && pos.X < z.lineNumberGrid.Size().Width {
		return CharPos{z.lineOffset + int(y/z.charSize.Height), 0.0, true}
	}
	return CharPos{z.lineOffset + int(y/z.charSize.Height), int(math32.Trunc(x / z.charSize.Width)), false}
}

func (z *ZGrid) MinSize() fyne.Size {
	return fyne.Size{Width: float32(z.lineNumberLen())*z.charSize.Width + float32(z.Columns)*z.charSize.Width,
		Height: float32(z.Lines) * z.charSize.Height}
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
	if z.Rows != nil && len(z.Rows) > z.lineOffset {
		z.grid.Rows = z.Rows[z.lineOffset:min(z.lineOffset+z.Lines, len(z.Rows))]
		for i, row := range z.grid.Rows {
			l := len(row.Cells)
			k := max(0, min(l-1, z.columnOffset))
			m := min(l, z.columnOffset+z.Columns)
			row.Cells = row.Cells[k:m]
			for j := range row.Cells {
				row.Cells[j] = widget.TextGridCell{Rune: row.Cells[j].Rune, Style: nil}
			}
			z.grid.SetRow(i, row)
		}
	}
	if z.ShowLineNumbers {
		z.lineNumberGrid.Hidden = false
		// add line numbers if necessary
		ll := strconv.Itoa(z.lineNumberLen())
		fmtStr := "%" + ll + "d "
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
	}
	if z.Tags.lineStylers != nil {
		for i := len(z.Tags.stylers) - 1; i >= 0; i-- {
			styler := z.Tags.lineStylers[i]
			interval, ok := z.Tags.Lookup(styler.Tag)
			if !ok {
				continue
			}
			log.Println(styler.Tag)
			z.maybeStyleLineRange(interval, styler.LineStyleFunc)
		}
	}
	if z.Tags.stylers != nil {
		for i := len(z.Tags.stylers) - 1; i >= 0; i-- {
			styler := z.Tags.stylers[i]
			interval, ok := z.Tags.Lookup(styler.Tag)
			if !ok {
				continue
			}
			log.Printf("styling %v\n", styler.Tag)
			z.maybeStyleRange(interval, styler.StyleFunc)
		}
	}
	z.lineNumberGrid.Refresh()
	z.grid.Refresh()
}

// curreentViewport is the char interval that is currently displayed
func (z *ZGrid) currentViewport() CharInterval {
	endLine := min(len(z.Rows)-1, z.lineOffset+z.Lines)
	endColumn := len(z.Rows[endLine].Cells) - 1
	return CharInterval{Start: CharPos{Line: z.lineOffset, Column: 0}, End: CharPos{Line: endLine, Column: endColumn}}
}

// maybeStyleRange styles the given char interval by style insofar as it is within
// the visible range of the underlying TextGrid (otherwise, nothing is done).
func (z *ZGrid) maybeStyleRange(interval CharInterval, styler TagStyleFunc) {
	viewPort := z.currentViewport()
	if interval.OutsideOf(viewPort) {
		return
	}
	rangeStart := MaxPos(viewPort.Start, interval.Start)
	rangeEnd := MinPos(viewPort.End, interval.End)
	for i := rangeStart.Line; i <= rangeEnd.Line; i++ {
		startCol := 0
		if i == rangeStart.Line {
			startCol = rangeStart.Column
		}
		endCol := len(z.Rows[i].Cells) - 1
		if i == rangeEnd.Line {
			endCol = min(endCol, rangeEnd.Column)
		}
		for j := startCol; j <= endCol; j++ {
			z.grid.SetCell(i-z.lineOffset, j, styler(z.grid.Rows[i-z.lineOffset].Cells[j]))
		}
	}
}

// maybeStyleLineRange is the same as maybeStyleRange except that a TagLineStyleFunc
// is used and only the style of the line as a whole is set.
func (z *ZGrid) maybeStyleLineRange(interval CharInterval, styler TagLineStyleFunc) {
	viewPort := z.currentViewport()
	if interval.OutsideOf(viewPort) {
		return
	}
	rangeStart := MaxPos(viewPort.Start, interval.Start)
	rangeEnd := MinPos(viewPort.End, interval.End)
	for i := rangeStart.Line; i <= rangeEnd.Line; i++ {
		z.grid.SetRowStyle(i, styler(z.grid.Rows[i].Style))
	}
}

func (z *ZGrid) lineNumberLen() int {
	s := strconv.Itoa(len(z.Rows))
	return len(s)
}

func (s *ZGrid) CreateRenderer() fyne.WidgetRenderer {
	return &zgridRenderer{zgrid: s}
}

type zgridRenderer struct {
	zgrid *ZGrid
}

func (r *zgridRenderer) Destroy() {}

func (r *zgridRenderer) Layout(size fyne.Size) {
	s := r.zgrid.MinSize()
	r.zgrid.border.Resize(fyne.Size{Width: s.Width - theme.Padding(), Height: s.Height})
	r.zgrid.border.Move(fyne.Position{X: theme.Padding(), Y: theme.Padding()})
	r.zgrid.scroll.Resize(fyne.Size{Width: theme.ScrollBarSize(), Height: r.zgrid.grid.MinSize().Height})
	r.zgrid.scroll.Move(fyne.Position{X: s.Width - theme.ScrollBarSize() - theme.Padding(), Y: theme.Padding() / 2})
	r.zgrid.background.Resize(fyne.Size{Width: s.Width + theme.Padding(), Height: s.Height - theme.Padding()})
}

func (r *zgridRenderer) MinSize() fyne.Size {
	return r.zgrid.MinSize()
}

func (r *zgridRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.zgrid.content}
}

func (r *zgridRenderer) Refresh() {
	r.zgrid.Refresh()
}
