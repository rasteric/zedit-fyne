package zedit

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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

type CaretMovement int

const (
	CaretDown CaretMovement = iota + 1
	CaretUp
	CaretLeft
	CaretRight
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
	Rows               []widget.TextGridRow
	Tags               *TagContainer
	Lines              int
	Columns            int
	ShowLineNumbers    bool
	ShowWhitespace     bool
	ScrollFactor       float32
	TabWidth           int // If set to 0 the fyne.DefaultTabWidth is used
	MinRefreshInterval time.Duration
	// text cursor
	DrawCaret            bool
	CaretBlinkDelay      time.Duration
	CaretOnDuration      time.Duration
	CaretOffDuration     time.Duration
	CaretPos             CharPos
	caretState           uint32
	hasCaretBlinking     uint32
	invertedDefaultStyle *widget.CustomTextGridStyle
	lastInteraction      time.Time
	caretBlinkCancel     func()
	// internal fields
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
	shortcuts       map[string]fyne.KeyboardShortcut
	handlers        map[string]func(z *ZGrid)
	keyHandlers     map[fyne.KeyName]func(z *ZGrid)
	// delayed refresh
	refresher     func()
	lastRefreshed time.Time
	// synchronization
	mutex sync.RWMutex
}

// ZGrid is like TextGrid but with an internal vertical scroll bar and an optional line number display.
func NewZGrid(columns, lines int) *ZGrid {
	bgcolor := gamut.Blends(theme.ForegroundColor(), theme.PrimaryColor(), 8)[2]
	fgcolor := theme.BackgroundColor()
	if fyne.CurrentApp().Settings().ThemeVariant() == theme.VariantDark {
		fgcolor = gamut.Tints(fgcolor, 8)[2]
	} else {
		fgcolor = gamut.Darker(fgcolor, 0.2)
	}
	z := ZGrid{Lines: lines, Columns: columns, grid: widget.NewTextGrid()}
	z.grid = widget.NewTextGrid()
	z.initInternalGrid()
	z.MinRefreshInterval = 10 * time.Millisecond
	z.shortcuts = make(map[string]fyne.KeyboardShortcut)
	z.handlers = make(map[string]func(z *ZGrid))
	z.keyHandlers = make(map[fyne.KeyName]func(z *ZGrid))
	z.CaretBlinkDelay = 3 * time.Second
	z.CaretOnDuration = 600 * time.Millisecond
	z.CaretOffDuration = 200 * time.Millisecond
	z.DrawCaret = true
	z.lastInteraction = time.Now()
	z.caretState = 1
	_, z.caretBlinkCancel = context.WithCancel(context.Background())
	z.invertedDefaultStyle = &widget.CustomTextGridStyle{FGColor: theme.InputBackgroundColor(), BGColor: theme.ForegroundColor()}
	z.Tags = NewTagContainer()
	z.ScrollFactor = 2.0
	z.defaultStyle = &widget.CustomTextGridStyle{FGColor: theme.ForegroundColor(), BGColor: theme.InputBackgroundColor()}
	z.lineNumberStyle = &widget.CustomTextGridStyle{FGColor: fgcolor, BGColor: bgcolor}
	z.background = canvas.NewRectangle(theme.InputBackgroundColor()) //theme.InputBackgroundColor())
	z.background.StrokeColor = theme.FocusColor()
	z.background.StrokeWidth = 4
	z.lineNumberGrid = widget.NewTextGrid()
	z.charSize = fyne.MeasureText("M", theme.TextSize(), fyne.TextStyle{Monospace: true})

	z.vSpacer = NewFixedSpacer(fyne.Size{Width: 0, Height: float32(z.Lines) * z.charSize.Height})

	z.scroll = container.NewScroll(z.vSpacer)
	z.scroll.OnScrolled = func(pos fyne.Position) {
		z.lineOffset = max(0, int(math32.Round(pos.Y/z.charSize.Height)))
		z.scroll.Offset = pos
		z.hasFocus = true
		z.Refresh()
	}
	z.border = container.NewBorder(nil, nil, z.lineNumberGrid, z.scroll, z.grid)
	z.content = container.New(layout.NewStackLayout(), z.background, z.border)
	z.Tags.AddStyler(TagStyler{Tag: SimpleTag{"selection"}, StyleFunc: z.SelectionStyleFunc(), DrawFullLine: true})
	z.SetText(" ")
	z.BlinkCaret(true)
	z.addDefaultShortcuts()
	return &z
}

// initInternalGrid initializes the internal grid (z.grid) to all spaces Lines x Columns.
// This grid is only used for display and may never change! It's like a VRAM fixed character display.
func (z *ZGrid) initInternalGrid() {
	z.grid.Rows = make([]widget.TextGridRow, z.Lines)
	for i := range z.grid.Rows {
		z.grid.Rows[i].Cells = make([]widget.TextGridCell, z.Columns)
		for j := range z.grid.Rows[i].Cells {
			z.grid.Rows[i].Cells[j].Rune = ' '
		}
	}
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

// RowText returns the text of the row at i, the empty string if i is out of bounds.
func (z *ZGrid) RowText(i int) string {
	if i < 0 || i >= len(z.Rows) {
		return ""
	}
	var sb strings.Builder
	for _, c := range z.Rows[i].Cells {
		sb.WriteRune(c.Rune)
	}
	return sb.String()
}

// SetCell sets the zgrid cell at the given line and column.
func (z *ZGrid) SetCell(pos CharPos, cell widget.TextGridCell) {
	z.Rows[pos.Line].Cells[pos.Column] = cell
}

// SetRow sets the zgrid row. If row is beyond the current size, empty rows are added accordingly.
func (z *ZGrid) SetRow(row int, content widget.TextGridRow) {
	if row >= len(z.Rows) {
		rows := make([]widget.TextGridRow, row-len(z.Rows)+1)
		z.Rows = append(z.Rows, rows...)
	}
	z.Rows[row] = content
}

// FindParagraphStart finds the start row of the paragraph in which row is located.
// If the row is 0, 0 is returned, otherwise this checks for the next line ending with lf and
// returns the row after it.
func (grid *ZGrid) FindParagraphStart(row int, lf rune) int {
	if row <= 0 {
		return 0
	}
	k := len(grid.Rows[row-1].Cells)
	if k == 0 {
		return row
	}
	if grid.Rows[row-1].Cells[k-1].Rune == lf {
		return row
	}
	return grid.FindParagraphStart(row-1, lf)
}

// SetRune sets the rune at a row, column. The indices must be valid.
func (grid *ZGrid) SetRune(pos CharPos, r rune) {
	grid.Rows[pos.Line].Cells[pos.Column].Rune = r
}

// SetStyle sets the style of the cell at row, column. The indices must be valid.
func (grid *ZGrid) SetStyle(pos CharPos, style widget.TextGridStyle) {
	grid.Rows[pos.Line].Cells[pos.Column].Style = style
}

// SetStyleRange sets the style to each cell in the given range.
func (grid *ZGrid) SetStyleRange(interval CharInterval, style widget.TextGridStyle) {
	for i := interval.Start.Line; i <= interval.End.Line; i++ {
		var s, e int
		if i == interval.Start.Line {
			s = interval.Start.Column
		}
		if i == interval.End.Line {
			e = interval.End.Column
		} else {
			e = len(grid.Rows[i].Cells) - 1
		}
		for j := s; j <= e; j++ {
			grid.Rows[i].Cells[j].Style = style
		}
	}
}

// Text returns the ZGrid's text as string.
func (grid *ZGrid) Text() string {
	var sb strings.Builder
	for i := range grid.Rows {
		for j := range grid.Rows[i].Cells {
			sb.WriteRune(grid.Rows[i].Cells[j].Rune)
		}
		if i < len(grid.Rows) {
			sb.WriteRune('\n')
		}
	}
	return sb.String()
}

// SetRowStyle sets the style of the row. The given row must be valid.
func (grid *ZGrid) SetRowStyle(row int, style widget.TextGridStyle) {
	grid.Rows[row].Style = style
}

// FindParagraphEnd finds the end row of the paragraph in which row is located.
// If row is the last row, then it is returned. Otherwise, it checks for the next row that
// ends in lf (which may be the row with which this method was called).
func (grid *ZGrid) FindParagraphEnd(row int, lf rune) int {
	if row >= len(grid.Rows)-1 {
		return row
	}
	k := len(grid.Rows[row].Cells)
	if k == 0 {
		return row
	}
	if grid.Rows[row].Cells[k-1].Rune == lf {
		return row
	}
	return grid.FindParagraphEnd(row+1, lf)
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
		return
	} else if pos.Line >= z.lineOffset+z.Lines-1 {
		z.ScrollDown()
		return
	}
	z.Refresh()
}

func (z *ZGrid) Cursor() desktop.Cursor {
	if z.selStart != nil {
		return desktop.TextCursor
	}
	return desktop.DefaultCursor
}

func (z *ZGrid) Tapped(evt *fyne.PointEvent) {
	z.RemoveSelection()
	pos := z.PosToCharPos(evt.Position)
	z.SetCaret(pos)
}

func (z *ZGrid) DragEnd() {
	z.selStart = nil
	z.selEnd = nil
}

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

// KEY HANDLING

func (z *ZGrid) TypedRune(rune) {
	z.lastInteraction = time.Now()
}

func (z *ZGrid) TypedKey(evt *fyne.KeyEvent) {
	if handler, ok := z.keyHandlers[evt.Name]; ok {
		z.lastInteraction = time.Now()
		handler(z)
	}
}

func (z *ZGrid) TypedShortcut(s fyne.Shortcut) {
	if handler, ok := z.handlers[s.ShortcutName()]; ok {
		z.lastInteraction = time.Now()
		handler(z)
	}
}

// AddhortcutHandler adds a keyboard shortcut to the grid.
func (z *ZGrid) AddShortcutHandler(s fyne.KeyboardShortcut, handler func(z *ZGrid)) {
	z.shortcuts[s.ShortcutName()] = s
	z.handlers[s.ShortcutName()] = handler
}

// RemoveShortcutHandler removes the keyboard shortcut handler with the given name.
func (z *ZGrid) RemoveShortcutHandler(s string) {
	delete(z.shortcuts, s)
	delete(z.handlers, s)
}

// AddKeyHandler adds a direct handler for the given key. Unlike AddShortcutHandler, a key handler
// is called whenever the key is pressed, even when no modifier is used.
func (z *ZGrid) AddKeyHandler(key fyne.KeyName, handler func(z *ZGrid)) {
	z.keyHandlers[key] = handler
}

// RemoveKeyHandler removes the handler for the given key.
func (z *ZGrid) RemoveKeyHandler(key fyne.KeyName) {
	delete(z.keyHandlers, key)
}

// addDefaultShortcuts adds a few standard shortcuts that will rarely need to be changed.
func (z *ZGrid) addDefaultShortcuts() {
	z.AddKeyHandler(fyne.KeyDown, func(z *ZGrid) {
		z.MoveCaret(CaretDown)
	})
	z.AddKeyHandler(fyne.KeyUp, func(z *ZGrid) {
		z.MoveCaret(CaretUp)
	})
	z.AddKeyHandler(fyne.KeyLeft, func(z *ZGrid) {
		z.MoveCaret(CaretLeft)
	})
	z.AddKeyHandler(fyne.KeyRight, func(z *ZGrid) {
		z.MoveCaret(CaretRight)
	})
}

// LAYOUT UPDATING

func (z *ZGrid) Refresh() {
	z.mutex.RLock()
	last := z.lastRefreshed
	fn := z.refresher
	interval := z.MinRefreshInterval
	z.mutex.RUnlock()
	if time.Now().Sub(last) >= interval {
		z.mutex.Lock()
		z.refresher = func() {
			z.lastRefreshed = time.Now()
			z.refreshProc()
		}
		z.mutex.Unlock()
		defer func() {
			z.mutex.Lock()
			z.refresher = nil
			z.mutex.Unlock()
		}()
		z.mutex.RLock()
		defer z.mutex.RUnlock()
		z.refresher()
		return
	}
	if fn != nil {
		return
	}
	go func() {
		time.Sleep(interval)
		z.Refresh()
	}()
}

func (z *ZGrid) refreshProc() {
	defer func() {
		z.lastInteraction = time.Now()
		z.maybeDrawCaret()
	}()
outer:
	for i := range z.Lines {
		if i+z.lineOffset >= len(z.Rows) {
			z.grid.Rows[i].Style = nil
			for j := range z.Columns {
				z.grid.Rows[i].Cells[j].Rune = ' '
				z.grid.Rows[i].Cells[j].Style = nil
			}
			continue outer
		}
	inner:
		for j := range z.Columns {
			if j+z.columnOffset >= len(z.Rows[i+z.lineOffset].Cells) {
				z.grid.Rows[i].Cells[j].Rune = ' '
				z.grid.Rows[i].Cells[j].Style = nil
				continue inner
			}
			z.grid.Rows[i].Cells[j].Rune = z.Rows[i+z.lineOffset].Cells[j+z.columnOffset].Rune
			z.grid.Rows[i].Cells[j].Style = nil
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
			z.maybeStyleRange(interval, styler.StyleFunc, styler.DrawFullLine)
		}
	}
	z.lineNumberGrid.Refresh()
	z.grid.Refresh()
}

// curreentViewport is the char interval that is currently displayed
func (z *ZGrid) currentViewport() CharInterval {
	endLine := min(len(z.Rows)-1, z.lineOffset+z.Lines-1)
	endColumn := len(z.Rows[endLine].Cells) - 1
	return CharInterval{Start: CharPos{Line: z.lineOffset, Column: 0},
		End: CharPos{Line: endLine, Column: endColumn}}
}

// CARET HANDLING

// drawCaret draws the text cursor if necessary.
func (z *ZGrid) maybeDrawCaret() bool {
	if !z.DrawCaret || !z.currentViewport().Contains(z.CaretPos) {
		return false
	}
	line := z.CaretPos.Line - z.lineOffset
	col := z.CaretPos.Column - z.columnOffset
	if line < 0 || line > len(z.grid.Rows)-1 || col < 0 || col > len(z.grid.Rows[line].Cells)-1 {
		return false
	}
	switch atomic.LoadUint32(&z.caretState) {
	case 2:
		z.grid.Rows[line].Cells[col].Style = z.invertedDefaultStyle
	case 1:
		z.grid.Rows[line].Cells[col].Style = z.defaultStyle
	default:
		z.grid.Rows[line].Cells[col].Style = z.Rows[z.CaretPos.Line].Cells[col].Style
	}
	z.grid.Refresh()
	return true
}

// BlinkCursor starts blinking the cursor or stops the cursor from blinking.
func (z *ZGrid) BlinkCaret(on bool) {
	if !on {
		z.caretBlinkCancel()
		atomic.StoreUint32(&z.hasCaretBlinking, 0)
		atomic.StoreUint32(&z.caretState, 2)
		z.maybeDrawCaret()
		return
	}
	atomic.StoreUint32(&z.hasCaretBlinking, 1)
	ctx, cancel := context.WithCancel(context.Background())
	z.caretBlinkCancel = cancel
	go func(ctx context.Context, z *ZGrid) {
		var oddTick bool
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if oddTick && time.Since(z.lastInteraction) > z.CaretBlinkDelay {
					atomic.StoreUint32(&z.caretState, 1)
					oddTick = false
					z.maybeDrawCaret()
					time.Sleep(z.CaretOffDuration)
				} else {
					atomic.StoreUint32(&z.caretState, 2)
					oddTick = true
					z.maybeDrawCaret()
					time.Sleep(z.CaretOnDuration)
				}
			}
		}
	}(ctx, z)
}

// HasBlinkingCaret returns true if the input cursor is blinking, false otherwise.
// use BlinkCursor to switch blinking on or off.
func (z *ZGrid) HasBlinkingCaret() bool {
	return atomic.LoadUint32(&z.hasCaretBlinking) > 0
}

// CaretOff switches the caret off temporarily. It returns true was blinking.
func (z *ZGrid) CaretOff() bool {
	blinking := z.HasBlinkingCaret()
	z.caretBlinkCancel()
	z.caretState = 0
	z.DrawCaret = false
	z.Refresh()
	return blinking
}

// CaretOn switches the caret on again after it has been switched off.
func (z *ZGrid) CaretOn(blinking bool) {
	z.DrawCaret = true
	z.caretState = 2
	z.BlinkCaret(blinking)
	z.Refresh()
}

func (z *ZGrid) SetCaret(pos CharPos) {
	drawCaret := z.DrawCaret
	blinking := z.CaretOff()
	defer func() {
		if drawCaret {
			z.CaretOn(blinking)
		}
	}()
	z.CaretPos = pos
}

// MoveCaret moves the caret according to the given movement direction, which may be one of
// CaretUp, CaretDown, CaretLeft, and CaretRight.
func (z *ZGrid) MoveCaret(dir CaretMovement) {
	drawCaret := z.DrawCaret
	blinking := z.CaretOff()
	defer func() {
		if drawCaret {
			z.CaretOn(blinking)
		}
	}()
	switch dir {
	case CaretDown:
		z.CaretPos = CharPos{Line: min(z.CaretPos.Line+1, len(z.Rows)-1), Column: z.CaretPos.Column}
		if z.CaretPos.Line == z.lineOffset+z.Lines {
			z.ScrollDown()
			return
		}
	case CaretUp:
		z.CaretPos = CharPos{Line: max(z.CaretPos.Line-1, 0), Column: z.CaretPos.Column}
		if z.CaretPos.Line == z.lineOffset-1 {
			z.ScrollUp()
			return
		}
	case CaretLeft:
		if z.CaretPos.Column == 0 {
			z.MoveCaret(CaretUp)
			z.CaretPos = CharPos{Line: z.CaretPos.Line, Column: len(z.Rows[z.CaretPos.Line].Cells) - 1}
			if z.CaretPos.Column > z.columnOffset+z.Columns {
				z.columnOffset = z.CaretPos.Column - z.Columns/2
			}
			return
		}
		z.CaretPos = CharPos{Line: z.CaretPos.Line, Column: z.CaretPos.Column - 1}
		if z.CaretPos.Column < z.columnOffset {
			z.ScrollLeft(z.Columns / 2)
		}
	case CaretRight:
		if z.CaretPos.Column >= len(z.Rows[z.CaretPos.Line].Cells)-1 {
			z.CaretPos = CharPos{Line: z.CaretPos.Line, Column: 0}
			z.columnOffset = 0
			z.MoveCaret(CaretDown)
			return
		}
		z.CaretPos = CharPos{Line: z.CaretPos.Line, Column: z.CaretPos.Column + 1}
		if z.CaretPos.Column >= z.columnOffset+z.Columns {
			z.ScrollRight(z.Columns / 2)
		}
	}
}

// STYLES

// maybeStyleRange styles the given char interval by style insofar as it is within
// the visible range of the underlying TextGrid (otherwise, nothing is done).
func (z *ZGrid) maybeStyleRange(interval CharInterval, styler TagStyleFunc, drawFullLine bool) {
	if z.currentViewport().OutsideOf(interval) {
		return
	}
	for i := range z.Lines {
		xi := i + z.lineOffset
		if xi >= len(z.Rows) {
			break
		}
		for j := range z.Columns {
			xj := j + z.columnOffset
			if interval.Contains(CharPos{Line: xi, Column: xj}) {
				if xj < len(z.Rows[xi].Cells) {
					z.grid.SetCell(i, j, styler(z.Rows[xi].Cells[xj]))
				} else if drawFullLine {
					z.grid.SetCell(i, j, styler(z.grid.Rows[i].Cells[j]))
				}
			}
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

// func (z *ZGrid) InvertStyle(style widget.TextGridStyle) widget.TextGridStyle {
// 	if style == nil {
// 		return z.invertedDefaultStyle
// 	}
// 	return &widget.CustomTextGridStyle{FGColor: style.BackgroundColor(), BGColor: style.TextColor()}
// }
