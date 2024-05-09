package zedit

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/chewxy/math32"
	"github.com/lucasb-eyer/go-colorful"
	"golang.org/x/exp/slices"
)

type CaretMovement int

const (
	CaretDown CaretMovement = iota + 1
	CaretUp
	CaretLeft
	CaretRight
	CaretHome
	CaretEnd
	CaretLineStart
	CaretLineEnd
	CaretHalfPageDown
	CaretHalfPageUp
	CaretPageDown
	CaretPageUp
)

// Config stores configuration information for an editor.
type Config struct {
	SelectionTag         Tag           // the tag used for marking selection ranges
	SelectionStyleFunc   TagStyleFunc  // style of the selection tag
	HighlightTag         Tag           // for trasnient highlighting (usually has a different style than selection)
	HighlightStyleFunc   TagStyleFunc  // style func for highlight
	MarkTag              Tag           // template for the mark tags
	MarkTags             []Tag         // a number of pre-configured tags used for marking text (default: 0..9 tags)
	MarkStyleFunc        TagStyleFunc  // mark style func, using the tag index to distinguish marks
	ErrorTag             Tag           // for errors
	ParenErrorTag        Tag           // for wrong right parenthesis
	ErrorStyleFunc       TagStyleFunc  // style of errors (default: theme error color)
	ShowLineNumbers      bool          // switches on or off the line number display, which is in a separate grid
	ShowWhitespace       bool          // show special glyphs for line endings (currently defunct)
	BlendFG              BlendMode     // how layers of color are blended/composited for text foreground
	BlendFGSwitched      bool          // whether to switch the colors while blending forground (sometimes makes a difference)
	BlendBG              BlendMode     // how layers of color are blended for background
	BlendBGSwitched      bool          // whether the colors are switched while blending background colors (sometimes makes a difference)
	HardLF               rune          // hard line feed character
	SoftLF               rune          // soft line feed character (subject to word-wrapping and deletion in text)
	ScrollFactor         float32       // speed of scrolling
	TabWidth             int           // If set to 0 the fyne.DefaultTabWidth is used
	MinRefreshInterval   time.Duration // minimum interval in ms to refresh display
	CharDrift            float32       // default 0.4, added to calculation per char when finding char position from x-position
	LineWrap             bool          // automatically wrap lines (default: true)
	SoftWrap             bool          // soft wrap lines, if not true wrapping inserst hard line feeds (default: true)
	HighlightParens      bool          // highlight parentheses and quotation marks (default: true)
	HighlightParenRange  bool          // highlight the whole range between matching parens (default: false)
	DrawCaret            bool          // if true, the caret is drawn, if false, the caret is handled but not drawn
	CaretBlinkDelay      time.Duration // period after last interaction before caret starts blinking
	CaretOnDuration      time.Duration // how long the caret is shown when blinking
	CaretOffDuration     time.Duration // how long a blinking caret is off
	ParagraphLineNumbers bool          // line numbers are based on paragraphs to take into account soft wrap
}

// NewConfig returns a new config with default values.
func NewConfig() *Config {
	z := &Config{}
	z.HighlightParens = true
	z.BlendFG = BlendOverlay
	z.BlendBG = BlendOverlay
	z.SelectionTag = NewTag("selection")
	z.SelectionStyleFunc = TagStyleFunc(func(tag Tag, c widget.TextGridCell) widget.TextGridCell {
		fg := theme.TextColor()
		bg := theme.SelectionColor()
		if c.Style != nil {
			if c.Style.TextColor() != nil {
				fg = BlendColors(z.BlendFG, z.BlendFGSwitched, c.Style.TextColor(), theme.ForegroundColor())
			}
			if c.Style.BackgroundColor() != nil {
				bg = BlendColors(z.BlendBG, z.BlendBGSwitched, c.Style.BackgroundColor(), theme.SelectionColor())
			}
		}
		selStyle := &widget.CustomTextGridStyle{FGColor: fg, BGColor: bg}
		return widget.TextGridCell{
			Rune:  c.Rune,
			Style: selStyle,
		}
	})
	z.HighlightTag = NewTag("highlight")
	z.HighlightStyleFunc = TagStyleFunc(func(tag Tag, c widget.TextGridCell) widget.TextGridCell {
		fg := theme.TextColor()
		bg := theme.PrimaryColor()
		if c.Style != nil {
			if c.Style.TextColor() != nil {
				fg = BlendColors(z.BlendFG, z.BlendFGSwitched, c.Style.TextColor(), theme.ForegroundColor())
			}
			if c.Style.BackgroundColor() != nil {
				bg = BlendColors(z.BlendBG, z.BlendBGSwitched, c.Style.BackgroundColor(), theme.PrimaryColor())
			}
		}
		selStyle := &widget.CustomTextGridStyle{FGColor: fg, BGColor: bg}
		return widget.TextGridCell{
			Rune:  c.Rune,
			Style: selStyle,
		}
	})
	z.ErrorTag = NewTag("error")
	z.ParenErrorTag = z.ErrorTag.Clone(1)
	z.ErrorStyleFunc = TagStyleFunc(func(tag Tag, c widget.TextGridCell) widget.TextGridCell {
		fg := theme.TextColor()
		bg := theme.ErrorColor()
		if c.Style != nil {
			if c.Style.TextColor() != nil {
				fg = BlendColors(z.BlendFG, z.BlendFGSwitched, c.Style.TextColor(), theme.TextColor())
			}
			if c.Style.BackgroundColor() != nil {
				bg = BlendColors(z.BlendBG, z.BlendBGSwitched, c.Style.BackgroundColor(), theme.ErrorColor())
			}
		}
		selStyle := &widget.CustomTextGridStyle{FGColor: fg, BGColor: bg}
		return widget.TextGridCell{
			Rune:  c.Rune,
			Style: selStyle,
		}
	})
	z.LineWrap = true
	z.SoftWrap = true
	z.HardLF = ' '
	z.SoftLF = '\r'
	z.CharDrift = 0.4
	z.MinRefreshInterval = 10 * time.Millisecond
	z.CaretBlinkDelay = 3 * time.Second
	z.CaretOnDuration = 600 * time.Millisecond
	z.CaretOffDuration = 200 * time.Millisecond
	z.DrawCaret = true
	z.ScrollFactor = 2.0
	// mark color and style
	z.MarkTags = make([]Tag, 10)
	z.MarkTag = NewTag("mark")
	for i := range z.MarkTags {
		z.MarkTags[i] = z.MarkTag.Clone(i)
		z.MarkTags[i].SetCallback(func(evt TagEvent, tag Tag, interval CharInterval) {
			log.Printf("Event: %v Mark: %v Interval: %v\n", evt, tag.Index(), interval)
		})
	}
	z.ParagraphLineNumbers = true
	return z
}

// Editor is the main editor widget. Even though some of its properties are public, this is merely
// for convenience and it's best to only modify it using methods. If there is no method for some
// operation, chances are high that direct manipulation of internals such as editor.Rows might
// break in the future.
type Editor struct {
	widget.BaseWidget
	Lines   int             // the number of lines displayed
	Columns int             // the number of columns displayed
	Rows    [][]rune        // the text
	Tags    *TagContainer   // all tags
	Styles  *StyleContainer // styles associated with tags
	Config  *Config         // editor configuration

	// internal fields
	caretPos             CharPos
	caretState           uint32
	hasCaretBlinking     uint32
	caretBlinkCancel     func()
	grid                 *widget.TextGrid
	scroll               *container.Scroll
	lineOffset           int
	columnOffset         int
	charSize             fyne.Size
	border               *fyne.Container
	lastInteraction      time.Time
	defaultStyle         EditorStyle
	invertedDefaultStyle EditorStyle
	lineNumberStyle      EditorStyle
	lineNumberGrid       *widget.TextGrid
	vSpacer              *FixedSpacer
	maxLineLen           int
	hasFocus             bool
	background           *canvas.Rectangle
	content              *fyne.Container
	selStart             *CharPos
	selEnd               *CharPos
	shortcuts            map[string]fyne.KeyboardShortcut
	handlers             map[string]func(z *Editor)
	keyHandlers          map[fyne.KeyName]func(z *Editor)
	canvas               fyne.Canvas
	// delayed refresh
	refresher     func()
	lastRefreshed time.Time
	// synchronization
	mutex sync.RWMutex
}

// NewEditor returns a new editor widget with fixed columns and lines, which is displayed in the given
// canvas object. The editor has default configuration.
func NewEditor(columns, lines int, c fyne.Canvas) *Editor {
	config := NewConfig()
	return NewEditorWithConfig(columns, lines, c, config)
}

// NewEditorWithConfig returns a new editor with fixed columns and lines, which is displayed in the given
// canvas and uses the given configuration. The Config must be obtained by NewConfig() to ensure
// all defaults are initialized but may be changed before calling this function.
func NewEditorWithConfig(columns, lines int, c fyne.Canvas, config *Config) *Editor {
	z := Editor{Lines: lines, Columns: columns + 1, grid: widget.NewTextGrid()}
	z.Config = config
	bgcolor := BlendColors(BlendColor, true, theme.TextColor(), theme.PrimaryColor())
	fgcolor := theme.BackgroundColor()
	z.Styles = NewStyleContainer()
	z.canvas = c
	z.grid = widget.NewTextGrid()
	z.initInternalGrid()
	z.shortcuts = make(map[string]fyne.KeyboardShortcut)
	z.handlers = make(map[string]func(z *Editor))
	z.keyHandlers = make(map[fyne.KeyName]func(z *Editor))
	z.lastInteraction = time.Now()
	z.caretState = 1
	z.Tags = NewTagContainer()
	_, z.caretBlinkCancel = context.WithCancel(context.Background())
	z.invertedDefaultStyle = &CustomEditorStyle{FGColor: theme.InputBackgroundColor(), BGColor: theme.ForegroundColor()}
	z.defaultStyle = &CustomEditorStyle{FGColor: theme.ForegroundColor(), BGColor: theme.InputBackgroundColor()}
	z.lineNumberStyle = &CustomEditorStyle{FGColor: fgcolor, BGColor: bgcolor}
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
		z.Focus()
	}
	z.border = container.NewBorder(nil, nil, z.lineNumberGrid, z.scroll, z.grid)
	z.content = container.New(layout.NewStackLayout(), z.background, z.border)
	// selection styler
	z.Styles.AddStyler(TagStyler{TagName: z.Config.SelectionTag.Name(),
		StyleFunc: z.Config.SelectionStyleFunc, DrawFullLine: true})
	z.Styles.AddStyler(TagStyler{TagName: z.Config.HighlightTag.Name(),
		StyleFunc: z.Config.HighlightStyleFunc, DrawFullLine: true})
	z.Styles.AddStyler(TagStyler{TagName: z.Config.ErrorTag.Name(),
		StyleFunc: z.Config.ErrorStyleFunc, DrawFullLine: false})
	// mark color and style

	col0, _ := colorful.MakeColor(color.RGBA{250, 190, 212, 255})
	col1, _ := colorful.MakeColor(color.RGBA{255, 215, 180, 255})
	col2, _ := colorful.MakeColor(color.RGBA{255, 250, 200, 255})
	col3, _ := colorful.MakeColor(color.RGBA{170, 255, 195, 255})
	col4, _ := colorful.MakeColor(color.RGBA{220, 190, 255, 255})
	col5, _ := colorful.MakeColor(color.RGBA{245, 130, 48, 255})
	col6, _ := colorful.MakeColor(color.RGBA{60, 180, 75, 255})
	col7, _ := colorful.MakeColor(color.RGBA{255, 225, 25, 255})
	col8, _ := colorful.MakeColor(color.RGBA{210, 245, 60, 255})
	col9, _ := colorful.MakeColor(color.RGBA{0, 130, 200, 255})

	markColors := []color.Color{
		col0,
		col1,
		col2,
		col3,
		col4,
		col5,
		col6,
		col7,
		col8,
		col9,
	}

	if fyne.CurrentApp().Settings().ThemeVariant() == theme.VariantDark {
		for i := range markColors {
			markColors[i] = BlendColors(BlendPhoenix, true, markColors[i], theme.InputBackgroundColor())
		}
	}

	markStyler := TagStyleFunc(func(tag Tag, c widget.TextGridCell) widget.TextGridCell {
		selStyle := &widget.CustomTextGridStyle{FGColor: theme.ForegroundColor(), BGColor: markColors[tag.Index()%10]}
		return widget.TextGridCell{
			Rune:  c.Rune,
			Style: selStyle,
		}
	})
	z.Styles.AddStyler(TagStyler{TagName: z.Config.MarkTag.Name(), StyleFunc: markStyler, DrawFullLine: true})
	z.SetText(" ")
	z.BlinkCaret(true)
	z.addDefaultShortcuts()
	return &z
}

// adjustScroll adjusts the internal spacer of the scroll bar. This method must be called after each
// change that might affect the number of rows.
func (z *Editor) adjustScroll() {
	z.vSpacer.SetHeight(float32(len(z.Rows)) * z.charSize.Height)
}

// initInternalGrid initializes the internal grid (z.grid) to all spaces Lines x Columns.
// This grid is only used for display and may never change! It's like a VRAM fixed character display.
func (z *Editor) initInternalGrid() {
	z.grid.Rows = make([]widget.TextGridRow, z.Lines)
	for i := range z.grid.Rows {
		z.grid.Rows[i].Cells = make([]widget.TextGridCell, z.Columns)
		for j := range z.grid.Rows[i].Cells {
			z.grid.Rows[i].Cells[j].Rune = ' '
			z.grid.Rows[i].Cells[j].Style = nil
		}
	}
}

// SetLineNumberStyle sets the style of the line number display in terms of an EditorStyle.
func (z *Editor) SetLineNumberStyle(style EditorStyle) {
	z.lineNumberStyle = style
}

// SetTopLine sets the zgrid to display starting with the given line number.
func (z *Editor) SetTopLine(x int) {
	z.lineOffset = x
	if z.scroll != nil {
		pos := z.scroll.Offset
		z.scroll.Offset = fyne.Position{X: pos.X, Y: max(0, z.charSize.Height*float32(z.lineOffset))}
	}
	z.Refresh()
	z.scroll.Refresh()
}

// CenterLineOnCaret adjusts the displayed lines such that the caret is in the center of the grid.
func (z *Editor) CenterLineOnCaret() {
	line := z.caretPos.Line
	z.SetTopLine(min(z.LastLine()-z.Lines+1, max(0, line-z.Lines/2)))
}

// LastLine returns the last line (0-indexed).
func (z *Editor) LastLine() int {
	return len(z.Rows) - 1
}

// LastColumn returns the last column of the given line (both 0-indexed).
func (z *Editor) LastColumn(n int) int {
	return len(z.Rows[n]) - 1
}

// LineText returns the text of line i, the empty string if i is out of bounds.
func (z *Editor) LineText(i int) string {
	if i < 0 || i > z.LastLine() {
		return ""
	}
	return string(z.Rows[i])
}

// SetRune sets the rune at the given line and column.
func (z *Editor) SetRune(pos CharPos, r rune) {
	z.Rows[pos.Line][pos.Column] = r
}

// SetLine sets the line text. If row is beyond the current size, empty rows are added accordingly.
func (z *Editor) SetLine(row int, content []rune) {
	if row > z.LastLine() {
		rows := makeEmptyRows(row - len(z.Rows) + 1)
		z.Rows = append(z.Rows, rows...)
	}
	z.Rows[row] = content
}

// FindParagraphStart finds the start row of the paragraph in which row is located.
// If the row is 0, 0 is returned, otherwise this checks for the next line ending with lf and
// returns the row after it.
func (z *Editor) FindParagraphStart(row int, lf rune) int {
	if row <= 0 {
		return 0
	}
	k := len(z.Rows[row-1])
	if k == 0 {
		return row
	}
	if z.Rows[row-1][k-1] == lf {
		return row
	}
	return z.FindParagraphStart(row-1, lf)
}

// FindParagraphEnd finds the end row of the paragraph in which row is located.
// If row is the last row, then it is returned. Otherwise, it checks for the next row that
// ends in lf (which may be the row with which this method was called).
func (grid *Editor) FindParagraphEnd(row int, lf rune) int {
	if row >= len(grid.Rows)-1 {
		return row
	}
	k := len(grid.Rows[row])
	if k == 0 {
		return row
	}
	if grid.Rows[row][k-1] == lf {
		return row
	}
	return grid.FindParagraphEnd(row+1, lf)
}

// Text returns the Editor's text as string. Both soft and hard linefeeds are replaced with rune '\n'.
func (z *Editor) Text() string {
	var sb strings.Builder
	for i := range z.Rows {
		for j := range z.Rows[i][:len(z.Rows[i])-1] {
			sb.WriteRune(z.Rows[i][j])
		}
		if i < len(z.Rows) {
			if z.Rows[i][len(z.Rows[i])-1] == z.Config.HardLF {
				sb.WriteRune('\n')
			} // TODO: Check - Should there be a ' ' with SoftLF? Or should it be dropped? There might be an ambiguity.
		}
	}
	return sb.String()
}

// SetMark marks a region. The given number must be a valid mark tag index.
func (z *Editor) SetMark(n int) {
	sel, hasSelection := z.Tags.Lookup(z.Config.SelectionTag)
	if !hasSelection {
		sel = CharInterval{Start: z.caretPos, End: z.caretPos}
	}
	z.Tags.Add(sel, z.Config.MarkTags[n])
	z.RemoveSelection()
	z.Refresh()
}

// Cut removes the selection text and corresponding tags.
func (z *Editor) Cut() {
	sel, ok := z.Tags.Lookup(z.Config.SelectionTag)
	if !ok {
		return
	}
	z.Delete(sel)
}

// ScrollDown scrolls down the editor's line display by one line.
func (z *Editor) ScrollDown() {
	li := min(len(z.Rows)-z.Lines/2, z.lineOffset+1)
	z.SetTopLine(li)
}

// ScrollUp scrolls up the editor's line display by one line.
func (z *Editor) ScrollUp() {
	li := max(0, z.lineOffset-1)
	z.SetTopLine(li)
}

// ScrollRight scrolls to the right by n chars but keeps some chars in display if n higher than the line.
func (z *Editor) ScrollRight(n int) {
	z.columnOffset = min(z.maxLineLen-z.Columns/2, z.columnOffset+n)
	z.Refresh()
}

// ScrollLeft scrolls to the left by n chars or until the first char if n is too large.
func (z *Editor) ScrollLeft(n int) {
	z.columnOffset = max(0, z.columnOffset-n)
	z.Refresh()
}

// FocusGained implements a Focusable.
func (z *Editor) FocusGained() {
	z.hasFocus = true
	z.background.StrokeColor = theme.FocusColor()
	z.background.Refresh()
	z.Refresh()
}

// FocusLost implements a Focusable.
func (z *Editor) FocusLost() {
	z.hasFocus = false
	z.background.StrokeColor = theme.BackgroundColor()
	z.background.Refresh()
	z.Refresh()
}

// Focus sets focus to the editor.
func (z *Editor) Focus() {
	z.canvas.Focus(z)
}

func (z *Editor) MouseIn(evt *desktop.MouseEvent) {}

func (z *Editor) MouseMoved(evt *desktop.MouseEvent) {}

func (z *Editor) MouseOut() {}

func (z *Editor) Scrolled(evt *fyne.ScrollEvent) {
	step := z.Config.ScrollFactor * (evt.Scrolled.DY / z.charSize.Height)
	z.lineOffset = min(len(z.Rows)-z.Lines/2, max(0, int(float32(z.lineOffset)-step)))
	z.scroll.Offset = fyne.Position{X: z.scroll.Offset.X, Y: float32(z.lineOffset) * z.charSize.Height}
	z.scroll.Refresh()
	z.Refresh()
}

func (z *Editor) Dragged(evt *fyne.DragEvent) {
	pos := z.PosToCharPos(evt.Position)
	if z.selStart == nil {
		z.selStart = &pos
		return
	}
	z.selEnd = &pos
	interval := CharInterval{Start: *z.selStart, End: *z.selEnd}.MaybeSwap()
	z.Tags.Upsert(z.Config.SelectionTag, interval)
	if pos.Line <= z.lineOffset {
		z.ScrollUp()
		return
	} else if pos.Line >= z.lineOffset+z.Lines-1 {
		z.ScrollDown()
		return
	}
	z.Refresh()
	z.Focus()
}

func (z *Editor) Cursor() desktop.Cursor {
	if z.selStart != nil {
		return desktop.TextCursor
	}
	return desktop.DefaultCursor
}

func (z *Editor) Tapped(evt *fyne.PointEvent) {
	pos := z.PosToCharPos(evt.Position)
	z.SetCaret(pos)
	z.Focus()
	z.RemoveSelection()
}

func (z *Editor) DoubleTapped(evt *fyne.PointEvent) {
	pos := z.PosToCharPos(evt.Position)
	z.SetCaret(pos)
	z.Focus()
	z.SelectWord(pos)
}

func (z *Editor) DragEnd() {
	z.selStart = nil
	z.selEnd = nil
}

// SELECTION HANDLING

// CurrentSelection returns the CharInterval if there is a non-empty selection marked,
// an empty CharInterval and false otherwise.
func (z *Editor) CurrentSelection() (CharInterval, bool) {
	sel, hasSelection := z.Tags.Lookup(z.Config.SelectionTag)
	if !hasSelection {
		return CharInterval{}, false
	}
	return sel, true
}

// SelectWord selects the word under pos if there is one, removes the selection in any case.
func (z *Editor) SelectWord(pos CharPos) {
	z.RemoveSelection()
	if pos.Line >= len(z.Rows) {
		return
	}
	if pos.Column >= len(z.Rows[pos.Line]) {
		return
	}
	var wStart, wEnd int
	j := pos.Column
	for i := pos.Column; i >= 0; i-- {
		c := z.Rows[pos.Line][i]
		if !(unicode.IsLetter(c) || unicode.IsNumber(c)) {
			wStart = j
			break
		}
		j = i
	}
	j = pos.Column
	for i := pos.Column; i < len(z.Rows[pos.Line]); i++ {
		c := z.Rows[pos.Line][i]
		if !(unicode.IsLetter(c) || unicode.IsNumber(c)) {
			wEnd = j
			break
		}
		j = i
	}
	if wEnd == 0 {
		return
	}
	z.selStart = &CharPos{Line: pos.Line, Column: wStart}
	z.selEnd = &CharPos{Line: pos.Line, Column: wEnd}
	z.Tags.Upsert(z.Config.SelectionTag, CharInterval{Start: *z.selStart, End: *z.selEnd})
	z.Refresh()
}

// Select the given char interval. The interval is sanitized before setting the selection.
func (z *Editor) Select(fromTo CharInterval) {
	fromTo = fromTo.Sanitize(z.LastPos())
	z.Tags.Upsert(z.Config.SelectionTag, fromTo)
	z.Refresh()
}

// SelectAll selects all text in the editor.
func (z *Editor) SelectAll() {
	fromTo := CharInterval{Start: CharPos{Line: 0, Column: 0}, End: z.LastPos()}
	z.Tags.Upsert(z.Config.SelectionTag, fromTo)
	z.Refresh()
}

// RemoveSelection removes the current selection, both the range returned by GetSelection
// and its graphical display.
func (z *Editor) RemoveSelection() {
	z.Tags.Delete(z.Config.SelectionTag)
	z.selStart = nil
	z.selEnd = nil
	z.Refresh()
}

// PosToCharPos converts an internal position of the widget in Fyne's pixel unit to a
// line, row pair.
func (z *Editor) PosToCharPos(pos fyne.Position) CharPos {
	x := pos.X - z.lineNumberGrid.Size().Width
	y := pos.Y
	if z.lineNumberGrid.Visible() && pos.X < z.lineNumberGrid.Size().Width {
		return CharPos{z.lineOffset + int(y/z.charSize.Height), 0, true}
	}
	row := z.lineOffset + int(y/z.charSize.Height)
	s := z.GetLineText(row)
	if z.columnOffset > 0 {
		s = substring(s, z.columnOffset, len(s))
	}
	column := z.findCharColumn(s, x)
	return CharPos{row, column + z.columnOffset, false}
}

// findCharColumn goes through a line explicitly and measures the position of each char in order to
// precisely determine a char position based on an x-coordinate. The original code was:
//
//	CharPos{z.lineOffset + int(y/z.charSize.Height), int(math32.Round(x / z.charSize.Width)), false}
//
// This is extremely imprecise because every character has a different width.
func (z *Editor) findCharColumn(s string, x float32) int {
	var sb strings.Builder
	offset := float32(0)
	for pos, char := range s {
		sb.WriteRune(char)
		size := fyne.MeasureText(sb.String(), theme.TextSize(), fyne.TextStyle{Monospace: true})
		if size.Width-offset > x {
			return max(0, pos-1)
		}
		offset = offset + z.Config.CharDrift // TODO CHANGE! ad hoc value
	}
	return len(s) - 1
}

// GetLineText obtains the text of a single line. The empty string is returned if there is no valid line.
func (z *Editor) GetLineText(row int) string {
	if row < 0 || row > z.LastLine() {
		return ""
	}
	return string(z.Rows[row])
}

func (z *Editor) MinSize() fyne.Size {
	if !z.Config.ShowLineNumbers {
		return fyne.Size{Width: float32(z.Columns) * z.charSize.Width,
			Height: float32(z.Lines) * z.charSize.Height}
	}
	return fyne.Size{Width: float32(z.lineNumberLen())*z.charSize.Width + float32(z.Columns)*z.charSize.Width,
		Height: float32(z.Lines) * z.charSize.Height}
}

// SetText sets the text in the editor to the given string, removing all tags in the process.
func (z *Editor) SetText(s string) {
	z.Tags.Clear()
	lines := strings.Split(s, "\n")
	// populate the text grid
	z.Rows = make([][]rune, 0)
	for _, line := range lines {
		r := []rune(line)
		r = append(r, z.Config.HardLF)
		newLines := make([][]rune, 0)
		if z.Config.LineWrap {
			newLines = append(newLines, z.wrapLine(r)...)
		} else {
			newLines = append(newLines, r)
		}
		z.Rows = append(z.Rows, newLines...)
		if len(z.Rows[len(z.Rows)-1]) > z.maxLineLen {
			z.maxLineLen = len(z.Rows[len(z.Rows)-1])
		}
	}
	z.Refresh()
}

// wrapLine word wraps a line of runes according to the editor settings for soft wrapping.
func (z *Editor) wrapLine(r []rune) [][]rune {
	var b strings.Builder
	lastGap := 0
	lineStart := 0
	i := 0
	c := 0
	hasSpace := false
	lines := make([][]rune, 0)
	for i = range r {
		c++
		if unicode.IsSpace(r[i]) {
			lastGap = i
			hasSpace = true
		}
		if c >= z.Columns {
			if !hasSpace {
				lastGap = i
			}
			for j := lineStart; j <= lastGap; j++ {
				b.WriteRune(r[j])
			}
			if z.Config.SoftWrap {
				b.WriteRune(z.Config.SoftLF)
			} else {
				b.WriteRune(z.Config.HardLF)
			}
			lines = append(lines, []rune(b.String()))
			b.Reset()
			lineStart = lastGap + 1
			hasSpace = false
			c = 0
		}
	}
	if lineStart < i {
		for j := lineStart; j <= i; j++ {
			b.WriteRune(r[j])
		}
		lines = append(lines, []rune(b.String()))
	}
	return lines
}

// PARAGRAPHS

// LineToPara returns the real paragraph number for a given 0-indexed row if there is one,
// false otherwise. The paragraph number is measured according to the hard LFs
// from the start of the document. If z.WordWrap is false, this function always
// returns the line + 1. However, if it is true, this function computes the
// paragraph number (indexed from 1) at the given line. This function is O(n) in the number of lines.
func (z *Editor) LineToPara(row int) (int, bool) {
	if !z.Config.LineWrap {
		return row + 1, true
	}
	if row == 0 {
		return 1, true
	}
	if row > z.LastLine() {
		return row + 1, false
	}
	c := 0
	for i := 0; i < row; i++ {
		if z.Rows[i][z.LastColumn(i)] == z.Config.HardLF {
			c++
		}
	}
	return c + 1, z.Rows[row-1][z.LastColumn(row-1)] == z.Config.HardLF
}

// ParaToLine returns the 0-indexed line number at which the given 1-index
// n-th paragraph starts and true if there is a paragraph with that index,
// 0 and false otherwise. This function is O(n) in the number of lines.
func (z *Editor) ParaToLine(paraNum int) (int, bool) {
	n := 0
	c := 0
	for i := range z.Rows {
		if z.Rows[i][z.LastColumn(i)] == z.Config.HardLF {
			n = i + 1
			c++
		}
		if c == paraNum-1 {
			return n, true
		}
	}
	return 0, false
}

// ParaCount counts the number of paragraphs, which is equivalent to the number of lines
// ending in HardLF + 1.
func (z *Editor) ParaCount() int {
	c := 0
	for i := range z.Rows {
		if z.Rows[i][z.LastColumn(i)] == z.Config.HardLF {
			c++
		}
	}
	return c
}

// KEY HANDLING

func (z *Editor) TypedRune(r rune) {
	z.lastInteraction = time.Now()
	z.Insert([]rune{r}, z.caretPos)
	z.MoveCaret(CaretRight)
}

func (z *Editor) TypedKey(evt *fyne.KeyEvent) {
	if handler, ok := z.keyHandlers[evt.Name]; ok {
		z.lastInteraction = time.Now()
		handler(z)
	}
}

func (z *Editor) TypedShortcut(s fyne.Shortcut) {
	if ks, ok := s.(fyne.KeyboardShortcut); ok {
		if handler, ok := z.handlers[GetKeyboardShortcutKey(ks)]; ok {
			z.lastInteraction = time.Now()
			handler(z)
		}
	}
}

// AddhortcutHandler adds a keyboard shortcut to the grid.
func (z *Editor) AddShortcutHandler(s fyne.KeyboardShortcut, handler func(z *Editor)) {
	z.shortcuts[GetKeyboardShortcutKey(s)] = s
	z.handlers[GetKeyboardShortcutKey(s)] = handler
}

// RemoveShortcutHandler removes the keyboard shortcut handler with the given key.
func (z *Editor) RemoveShortcutHandler(s string) {
	delete(z.shortcuts, s)
	delete(z.handlers, s)
}

// AddKeyHandler adds a direct handler for the given key. Unlike AddShortcutHandler, a key handler
// is called whenever the key is pressed, even when no modifier is used.
func (z *Editor) AddKeyHandler(key fyne.KeyName, handler func(z *Editor)) {
	z.keyHandlers[key] = handler
}

// RemoveKeyHandler removes the handler for the given key.
func (z *Editor) RemoveKeyHandler(key fyne.KeyName) {
	delete(z.keyHandlers, key)
}

// addDefaultShortcuts adds a few standard shortcuts that will rarely need to be changed.
func (z *Editor) addDefaultShortcuts() {
	z.AddKeyHandler(fyne.KeyDown, func(z *Editor) {
		z.MoveCaret(CaretDown)
	})
	z.AddKeyHandler(fyne.KeyUp, func(z *Editor) {
		z.MoveCaret(CaretUp)
	})
	z.AddKeyHandler(fyne.KeyLeft, func(z *Editor) {
		z.MoveCaret(CaretLeft)
	})
	z.AddKeyHandler(fyne.KeyRight, func(z *Editor) {
		z.MoveCaret(CaretRight)
	})
	z.AddKeyHandler(fyne.KeyHome, func(z *Editor) {
		z.MoveCaret(CaretHome)
	})
	z.AddKeyHandler(fyne.KeyEnd, func(z *Editor) {
		z.MoveCaret(CaretEnd)
	})
	z.AddKeyHandler(fyne.KeyPageDown, func(z *Editor) {
		z.MoveCaret(CaretHalfPageDown)
	})
	z.AddKeyHandler(fyne.KeyPageUp, func(z *Editor) {
		z.MoveCaret(CaretHalfPageUp)
	})
	z.AddKeyHandler(fyne.KeyBackspace, func(z *Editor) {
		z.Backspace()
	})
	z.AddKeyHandler(fyne.KeyDelete, func(z *Editor) {
		z.Delete1()
	})
	z.AddKeyHandler(fyne.KeyReturn, func(z *Editor) {
		z.Return()
	})
	// shortcuts
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.KeyPageDown, Modifier: fyne.KeyModifierControl},
		func(z *Editor) {
			z.MoveCaret(CaretPageDown)
		})
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.KeyPageUp, Modifier: fyne.KeyModifierControl},
		func(z *Editor) {
			z.MoveCaret(CaretPageUp)
		})
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.KeyX, Modifier: fyne.KeyModifierControl},
		func(z *Editor) {
			z.Cut()
		})
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.Key1, Modifier: fyne.KeyModifierAlt},
		func(z *Editor) {
			z.SetMark(1)
		})
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.Key2, Modifier: fyne.KeyModifierAlt},
		func(z *Editor) {
			z.SetMark(2)
		})
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.Key3, Modifier: fyne.KeyModifierAlt},
		func(z *Editor) {
			z.SetMark(3)
		})
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.Key4, Modifier: fyne.KeyModifierAlt},
		func(z *Editor) {
			z.SetMark(4)
		})
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.Key5, Modifier: fyne.KeyModifierAlt},
		func(z *Editor) {
			z.SetMark(5)
		})
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.Key6, Modifier: fyne.KeyModifierAlt},
		func(z *Editor) {
			z.SetMark(6)
		})
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.Key7, Modifier: fyne.KeyModifierAlt},
		func(z *Editor) {
			z.SetMark(7)
		})
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.Key8, Modifier: fyne.KeyModifierAlt},
		func(z *Editor) {
			z.SetMark(8)
		})
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.Key9, Modifier: fyne.KeyModifierAlt},
		func(z *Editor) {
			z.SetMark(9)
		})
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.Key0, Modifier: fyne.KeyModifierAlt},
		func(z *Editor) {
			z.SetMark(0)
		})
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.KeyA, Modifier: fyne.KeyModifierControl},
		func(z *Editor) {
			z.SelectAll()
		})
}

// AddEmacsShortcuts adds some (very basic) Emacs shortcuts but some with Super key as modifier instead of Ctrl
// in order not to interfere with standard platform keyboard shortcuts.
func (z *Editor) AddEmacsShortcuts() {
	// shortcuts
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.KeyE, Modifier: fyne.KeyModifierAlt},
		func(z *Editor) {
			z.MoveCaret(CaretLineEnd)
		})
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.KeyQ, Modifier: fyne.KeyModifierAlt},
		func(z *Editor) {
			z.MoveCaret(CaretLineStart)
		})
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.KeyN, Modifier: fyne.KeyModifierAlt},
		func(z *Editor) {
			z.MoveCaret(CaretDown)
		})
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.KeyP, Modifier: fyne.KeyModifierAlt},
		func(z *Editor) {
			z.MoveCaret(CaretUp)
		})
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.KeyF, Modifier: fyne.KeyModifierAlt},
		func(z *Editor) {
			z.MoveCaret(CaretRight)
		})
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.KeyB, Modifier: fyne.KeyModifierAlt},
		func(z *Editor) {
			z.MoveCaret(CaretLeft)
		})
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.KeyV, Modifier: fyne.KeyModifierAlt},
		func(z *Editor) {
			z.MoveCaret(CaretHalfPageDown)
		})
	z.AddShortcutHandler(&desktop.CustomShortcut{KeyName: fyne.KeyV, Modifier: fyne.KeyModifierAlt | fyne.KeyModifierShift},
		func(z *Editor) {
			z.MoveCaret(CaretHalfPageUp)
		})
}

// LAYOUT UPDATING

func (z *Editor) Refresh() {
	z.mutex.RLock()
	last := z.lastRefreshed
	fn := z.refresher
	interval := z.Config.MinRefreshInterval
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

func (z *Editor) refreshProc() {
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
			if j+z.columnOffset >= len(z.Rows[i+z.lineOffset]) {
				z.grid.Rows[i].Cells[j].Rune = ' '
				z.grid.Rows[i].Cells[j].Style = nil
				continue inner
			}
			z.grid.Rows[i].Cells[j].Rune = z.Rows[i+z.lineOffset][j+z.columnOffset]
			z.grid.Rows[i].Cells[j].Style = nil
		}
	}

	if z.Config.ShowLineNumbers {
		z.lineNumberGrid.Hidden = false
		// add line numbers if necessary
		ll := strconv.Itoa(z.lineNumberLen())
		fmtStr := "%" + ll + "d "
		paraLineNo := z.Config.ParagraphLineNumbers
		showLineNo := !paraLineNo
		for i := 0; i < z.Lines; i++ {
			var s []rune
			if paraLineNo {
				var lino int
				lino, showLineNo = z.LineToPara(z.lineOffset + i)
				s = []rune(fmt.Sprintf(fmtStr, lino))
			} else {
				s = []rune(fmt.Sprintf(fmtStr, z.lineOffset+i+1))
			}
			for j := 0; j < len(s); j++ {
				if showLineNo && z.lineOffset+i <= z.LastLine() {
					z.lineNumberGrid.SetCell(i, j, widget.TextGridCell{Rune: s[j], Style: z.lineNumberStyle})
				} else {
					z.lineNumberGrid.SetCell(i, j, widget.TextGridCell{Rune: ' ', Style: z.lineNumberStyle})
				}
			}
		}
	}

	stylers := z.Styles.Stylers()
	if stylers != nil {
		for i := len(stylers) - 1; i >= 0; i-- {
			tags, ok := z.Tags.TagsByName(stylers[i].TagName)
			if !ok {
				continue
			}
			loop := tags.Iter()
			for {
				tag, ok := loop.Next()
				if !ok {
					break
				}
				interval, ok := z.Tags.Lookup(tag)
				if !ok {
					continue
				}
				z.maybeStyleRange(tag, interval, stylers[i].StyleFunc, stylers[i].DrawFullLine)
			}
		}
	}
	z.adjustScroll()
	z.lineNumberGrid.Refresh()
	z.grid.Refresh()
}

// curreentViewport is the char interval that is currently displayed
func (z *Editor) currentViewport() CharInterval {
	endLine := min(len(z.Rows)-1, z.lineOffset+z.Lines-1)
	endColumn := len(z.Rows[endLine]) - 1
	return CharInterval{Start: CharPos{Line: z.lineOffset, Column: 0},
		End: CharPos{Line: endLine, Column: endColumn}}
}

// CARET HANDLING

// drawCaret draws the text cursor if necessary.
func (z *Editor) maybeDrawCaret() bool {
	if !z.Config.DrawCaret {
		return false
	}
	line := z.caretPos.Line - z.lineOffset
	if line < 0 || line > z.Lines-1 {
		return false
	}
	line = SafePositiveValue(line, len(z.grid.Rows)-1)
	col := z.caretPos.Column - z.columnOffset
	if col > z.Columns-1 {
		return false
	}
	col = SafePositiveValue(col, len(z.grid.Rows[line].Cells)-1)
	switch atomic.LoadUint32(&z.caretState) {
	case 2:
		z.grid.Rows[line].Cells[col].Style = z.invertedDefaultStyle
	default:
		z.grid.Rows[line].Cells[col].Style = z.defaultStyle
	}
	z.grid.Refresh()
	return true
}

// BlinkCursor starts blinking the cursor or stops the cursor from blinking.
func (z *Editor) BlinkCaret(on bool) {
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
	go func(ctx context.Context, z *Editor) {
		var oddTick bool
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if oddTick && time.Since(z.lastInteraction) > z.Config.CaretBlinkDelay {
					atomic.StoreUint32(&z.caretState, 1)
					oddTick = false
					z.maybeDrawCaret()
					time.Sleep(z.Config.CaretOffDuration)
				} else {
					atomic.StoreUint32(&z.caretState, 2)
					oddTick = true
					z.maybeDrawCaret()
					time.Sleep(z.Config.CaretOnDuration)
				}
			}
		}
	}(ctx, z)
}

// HasBlinkingCaret returns true if the input cursor is blinking, false otherwise.
// use BlinkCursor to switch blinking on or off.
func (z *Editor) HasBlinkingCaret() bool {
	return atomic.LoadUint32(&z.hasCaretBlinking) > 0
}

// CaretOff switches the caret off temporarily. It returns true was blinking.
func (z *Editor) CaretOff() bool {
	blinking := z.HasBlinkingCaret()
	z.caretBlinkCancel()
	z.caretState = 0
	z.Config.DrawCaret = false
	z.Refresh()
	return blinking
}

// CaretOn switches the caret on again after it has been switched off.
func (z *Editor) CaretOn(blinking bool) {
	z.Config.DrawCaret = true
	z.caretState = 2
	z.BlinkCaret(blinking)
	z.Refresh()
}

// handleCaretEvent emits an event for all tags whose range contains pos1 as long as it doesn't also contain pos2.
// Tags without callback function are ignored.
func (z *Editor) handleCaretEvent(evt TagEvent, pos1, pos2 CharPos) {
	tags, ok := z.Tags.LookupRange(CharInterval{Start: pos1, End: pos1})
	if ok {
		for _, tag := range tags {
			cb := tag.Callback()
			if cb == nil {
				continue
			}
			if interval, ok := z.Tags.Lookup(tag); ok {
				if interval.Contains(pos2) {
					continue
				}
				cb(evt, tag, interval)
			}
		}
	}
}

// GetCaret returns the current caret position.
func (z *Editor) GetCaret() CharPos {
	return z.caretPos
}

// SetCaret sets the current caret position, taking care of paren highlighting
// and caret events but without scrolling or refreshing the display.
func (z *Editor) SetCaret(pos CharPos) {
	// handle caret leave event
	z.handleCaretEvent(CaretLeaveEvent, z.caretPos, pos)

	// handle caret itself
	oldPos := z.caretPos
	drawCaret := z.Config.DrawCaret
	blinking := z.CaretOff()
	defer func() {
		if drawCaret {
			z.CaretOn(blinking)
		}
	}()
	z.caretPos = pos
	z.maybeHighlightParen()

	// handle caret enter event
	z.handleCaretEvent(CaretEnterEvent, pos, oldPos)
}

func (z *Editor) maybeHighlightParen() {
	z.Tags.DeleteByName(z.Config.HighlightTag.Name())
	z.Tags.Delete(z.Config.ParenErrorTag)
	if !z.Config.HighlightParens {
		return
	}
	pos, ok := z.PrevPos(z.caretPos)
	if !ok {
		return
	}
	r, ok := z.CharAt(pos)
	if !ok {
		return
	}
	if !(IsRightParen(r) || IsQuotationMark(r)) {
		return
	}
	current, ok := z.PrevPos(pos)
	if !ok {
		z.MarkErrorParen(CharInterval{Start: pos, End: pos})
		return
	}
	var match rune
	switch r {
	case ')':
		match = '('
	case ']':
		match = '['
	case '}':
		match = '{'
	default:
		match = r
	}
	openParens := 0
	if IsRightParen(r) {
		openParens = 1
	}
	lpos, ok := z.FindRune(current, true, func(c rune) bool {
		if IsRightParen(c) {
			openParens++
		} else if IsLeftParen(c) {
			openParens--
		}
		return c == match && openParens == 0
	})
	if !ok {
		z.MarkErrorParen(CharInterval{Start: pos, End: pos})
		return
	}
	if z.Config.HighlightParenRange {
		z.Highlight(CharInterval{Start: lpos, End: pos})
		return
	}
	z.Highlight(CharInterval{Start: pos, End: pos})
	z.Highlight(CharInterval{Start: lpos, End: lpos})
}

// FindRune searches one rune forward or backward, using searchFunc and returns the matching rune's position
// and true, or (0,0) and false. pos is included in the search.
func (z *Editor) FindRune(pos CharPos, backward bool, searchFunc func(c rune) bool) (CharPos, bool) {
	for {
		c, ok := z.CharAt(pos)
		if !ok {
			break
		}
		if searchFunc(c) {
			return pos, true
		}
		if backward {
			pos, ok = z.PrevPos(pos)
		} else {
			pos, ok = z.NextPos(pos)
		}
		if !ok {
			break
		}
	}
	return CharPos{}, false
}

// Highlight highlights a char interval using the default highlight tag and style. This method
// does not remove any previous highlights.
func (z *Editor) Highlight(interval CharInterval) {
	tag := z.Tags.CloneTag(z.Config.HighlightTag)
	z.Tags.Add(interval, tag)
}

// MarkError marks an error at a given range or removes it. Any existing error in the interval is
// removed. This is a quick and dirty solution. For full syntax coloring, it may be better to use
// a custom function instead of this one.
func (z *Editor) MarkErrorParen(interval CharInterval) {
	z.Tags.Delete(z.Config.ParenErrorTag)
	z.Tags.Add(interval, z.Config.ParenErrorTag)
}

// CharAt returns the unicode glyph at the given position.
func (z *Editor) CharAt(pos CharPos) (rune, bool) {
	if len(z.Rows) == 0 {
		return unicode.ReplacementChar, false
	}
	if pos.Line < 0 || pos.Column < 0 {
		return unicode.ReplacementChar, false
	}
	if CmpPos(pos, z.LastPos()) > 0 {
		return unicode.ReplacementChar, false
	}
	if pos.Column > z.LastColumn(pos.Line) {
		return unicode.ReplacementChar, false
	}
	return z.Rows[pos.Line][pos.Column], true
}

// MoveCaret moves the caret according to the given movement direction, which may be one of
// CaretUp, CaretDown, CaretLeft, and CaretRight.
func (z *Editor) MoveCaret(dir CaretMovement) {
	drawCaret := z.Config.DrawCaret
	blinking := z.CaretOff()
	defer func() {
		if drawCaret {
			z.CaretOn(blinking)
			z.maybeDrawCaret()
			z.maybeHighlightParen()
		}
	}()
	oldPos := z.caretPos
	defer func(oldPos CharPos) {
		z.handleCaretEvent(CaretEnterEvent, z.caretPos, oldPos)
	}(oldPos)
	var newPos CharPos
	switch dir {
	case CaretDown:
		newPos = CharPos{Line: min(z.caretPos.Line+1, len(z.Rows)-1), Column: z.caretPos.Column}
		z.handleCaretEvent(CaretLeaveEvent, oldPos, newPos)
		z.caretPos = newPos
		if z.caretPos.Line == z.lineOffset+z.Lines {
			z.ScrollDown()
			return
		}
	case CaretUp:
		newPos = CharPos{Line: max(z.caretPos.Line-1, 0), Column: z.caretPos.Column}
		z.handleCaretEvent(CaretLeaveEvent, oldPos, newPos)
		z.caretPos = newPos
		if z.caretPos.Line == z.lineOffset-1 {
			z.ScrollUp()
			return
		}
	case CaretLeft:
		if z.caretPos.Column == 0 {
			if z.caretPos.Line == 0 {
				return
			}
			z.MoveCaret(CaretUp)
			newPos = CharPos{Line: z.caretPos.Line, Column: len(z.Rows[z.caretPos.Line]) - 1}
			z.handleCaretEvent(CaretLeaveEvent, oldPos, newPos)
			z.caretPos = newPos
			if z.caretPos.Column > z.columnOffset+z.Columns {
				z.columnOffset = z.caretPos.Column - z.Columns/2
			}
			return
		}
		newPos = CharPos{Line: z.caretPos.Line, Column: z.caretPos.Column - 1}
		z.handleCaretEvent(CaretLeaveEvent, oldPos, newPos)
		z.caretPos = newPos
		if z.caretPos.Column < z.columnOffset {
			z.ScrollLeft(z.Columns / 2)
		}
	case CaretRight:
		if z.caretPos.Column >= len(z.Rows[z.caretPos.Line])-1 {
			z.caretPos = CharPos{Line: z.caretPos.Line, Column: 0}
			z.columnOffset = 0
			z.MoveCaret(CaretDown)
			return
		}
		newPos = CharPos{Line: z.caretPos.Line, Column: z.caretPos.Column + 1}
		z.handleCaretEvent(CaretLeaveEvent, oldPos, newPos)
		z.caretPos = newPos
		if z.caretPos.Column >= z.columnOffset+z.Columns {
			z.ScrollRight(z.Columns / 2)
		}
	case CaretHome:
		newPos = CharPos{Line: 0, Column: 0}
		z.handleCaretEvent(CaretLeaveEvent, oldPos, newPos)
		z.caretPos = newPos
		z.SetTopLine(0)
	case CaretEnd:
		newTop := z.LastLine() - z.Lines + 1
		z.SetTopLine(newTop)
		newPos = CharPos{Line: z.LastLine(), Column: z.LastColumn(z.LastLine())}
		z.handleCaretEvent(CaretLeaveEvent, oldPos, newPos)
		z.caretPos = newPos
	case CaretLineStart:
		newPos = CharPos{Line: z.caretPos.Line, Column: 0}
		if z.columnOffset > 0 {
			z.columnOffset = 0
		}
		z.handleCaretEvent(CaretLeaveEvent, oldPos, newPos)
		z.caretPos = newPos
	case CaretLineEnd:
		newPos = CharPos{Line: z.caretPos.Line, Column: z.LastColumn(z.caretPos.Line)}
		z.handleCaretEvent(CaretLeaveEvent, oldPos, newPos)
		z.caretPos = newPos
		if z.caretPos.Column >= z.columnOffset+z.Columns {
			z.ScrollRight(z.Columns / 2)
		}
	case CaretHalfPageDown:
		newLine := min(z.LastLine(), z.caretPos.Line+z.Lines/2)
		newPos = CharPos{Line: newLine, Column: z.caretPos.Column}
		z.handleCaretEvent(CaretLeaveEvent, oldPos, newPos)
		z.caretPos = newPos
		if newLine > z.lineOffset+z.Lines-1 {
			z.CenterLineOnCaret()
		}
	case CaretHalfPageUp:
		newLine := max(0, z.caretPos.Line-z.Lines/2)
		newPos = CharPos{Line: newLine, Column: z.caretPos.Column}
		z.handleCaretEvent(CaretLeaveEvent, oldPos, newPos)
		z.caretPos = newPos
		if newLine < z.lineOffset {
			z.CenterLineOnCaret()
		}
	case CaretPageDown:
		newLine := min(z.LastLine(), z.caretPos.Line+z.Lines)
		newPos = CharPos{Line: newLine, Column: z.caretPos.Column}
		z.handleCaretEvent(CaretLeaveEvent, oldPos, newPos)
		z.caretPos = newPos
		if newLine > z.lineOffset+z.Lines-1 {
			z.CenterLineOnCaret()
		}
	case CaretPageUp:
		newLine := max(0, z.caretPos.Line-z.Lines)
		newPos = CharPos{Line: newLine, Column: z.caretPos.Column}
		z.handleCaretEvent(CaretLeaveEvent, oldPos, newPos)
		z.caretPos = newPos
		if newLine < z.lineOffset {
			z.CenterLineOnCaret()
		}
	}
}

// INSERT with soft wrap

// Insert inserts an array of TextGridCells at row, col, optionally soft wrapping it and using
// hardLF and softLF as hard and soft line feed characters. The cursor position and tags
// are updated automatically by this method.
func (z *Editor) Insert(r []rune, pos CharPos) {
	startRow := z.FindParagraphStart(pos.Line, z.Config.HardLF)
	endRow := z.FindParagraphEnd(pos.Line, z.Config.HardLF)
	// endRowLastColumn := len(z.Rows[endRow].Cells) - 1
	rows := make([][]rune, (endRow-startRow)+1)
	for i := range rows {
		rows[i] = z.Rows[i+startRow]
	}
	k := pos.Line - startRow // the row into which we insert
	line := rows[k]
	lenLine := len(line)
	lenInsert := len(r)
	n := lenLine + lenInsert
	newLine := make([]rune, 0, n)
	if pos.Column >= lenLine {
		newLine = append(newLine, line...)
		newLine = append(newLine, r...)
	} else if pos.Column == 0 {
		newLine = append(newLine, r...)
		newLine = append(newLine, line...)
	} else {
		newLine = append(newLine, line[:pos.Column]...)
		newLine = append(newLine, r...)
		newLine = append(newLine, line[pos.Column:lenLine]...)
	}
	rows[k] = newLine

	// adjust tags
	tags, ok := z.Tags.LookupRange(z.ToEnd(pos))
	if ok {
		for _, tag := range tags {
			if tag == nil {
				continue
			}
			interval, ok := z.Tags.Lookup(tag)
			if !ok {
				log.Printf(`WARN tag "%v" has no associated interval [Insert]\n`, tag.Name())
				continue // non-fatal error, ignore
			}
			if interval.Start.Line == pos.Line {
				// the tag's interval starts on the same line as we're inserting
				// this is the only case to consider before word wrapping
				if pos.Column < interval.Start.Column {
					// we insert before, so shift interval by text inserted
					var endPos CharPos
					if interval.End.Line == pos.Line {
						endPos = CharPos{Line: interval.End.Line, Column: interval.End.Column + lenInsert}
					} else {
						endPos = interval.End
					}
					newInterval := CharInterval{Start: CharPos{Line: interval.Start.Line,
						Column: interval.Start.Column + lenInsert}, End: endPos}
					z.Tags.Upsert(tag, newInterval)
				}
			}
		}
	}
	// end adjust tags

	var cline, ccol int
	cline = pos.Line - startRow
	ccol = pos.Column
	if z.Config.LineWrap {
		rows, cline, ccol = z.WordWrapRows(rows, z.Columns, z.Config.SoftWrap, z.Config.HardLF, z.Config.SoftLF,
			cline, ccol, startRow, tags, pos)
	}
	z.caretPos = CharPos{Line: cline + startRow, Column: ccol}
	lineDelta := len(rows) - (endRow - startRow + 1)
	// check if we need to delete rows
	if lineDelta < 0 {
		z.Rows = slices.Delete(z.Rows, startRow+len(rows), endRow+1)
		z.adjustTagLines(tags, lineDelta, pos)
	}
	// check if we need to insert additional rows
	if lineDelta > 0 {
		newRows := makeEmptyRows(len(rows) - (endRow - startRow + 1))
		z.Rows = slices.Insert(z.Rows, endRow+1, newRows...)
		z.adjustTagLines(tags, lineDelta, pos)
	}
	for i := range rows {
		z.Rows[i+startRow] = rows[i]
	}
}

// adjustTagLines adjusts the given tags based on the given lineDelta, which represents the number of lines added
// or removed when a paragraph is reflown. When the insertPos is before the tags interval, the start and end
// of the tag interval need to be adjusted by lineDelta lines. Otherwise, the only the end line needs to be adjusted.
func (z *Editor) adjustTagLines(tags []Tag, lineDelta int, insertPos CharPos) {
	for _, tag := range tags {
		if tag == nil {
			continue
		}
		interval, ok := z.Tags.Lookup(tag)
		if !ok {
			log.Printf(`WARN tag "%v" has no associated interval [adjustTags]\n`, tag.Name())
			continue // non-fatal error, ignore
		}
		if insertPos.Line == interval.End.Line {
			continue
		}
		newInterval := interval
		newInterval.End = CharPos{Line: interval.End.Line + lineDelta, Column: interval.End.Column}
		if insertPos.Line < interval.Start.Line {
			newInterval.Start = CharPos{Line: interval.Start.Line + lineDelta, Column: interval.Start.Column}
		}
		z.Tags.Upsert(tag, newInterval)
	}
}

// DELETE with soft wrap

// Delete deletes a range of characters, optionally soft wrapping the paragraph with given hardLF
// and softLF runes as hard and soft line feed characters.
func (z *Editor) Delete(fromTo CharInterval) {
	z.RemoveSelection()
	fromTo = fromTo.Sanitize(z.LastPos())

	// We look up the tags starting at or after the deletion start position.
	tags, ok := z.Tags.LookupRange(z.ToEnd(fromTo.Start))
	if !ok {
		log.Println("NO TAG FOUND")
	}
	// The tags are now adjusted for the deletion interval (many cases to consider). Word wrapping is handled separately.
	if ok {
		for _, tag := range tags {
			if tag == nil {
				log.Println(`WARN tag is nil [Delete]`)
				continue
			}
			interval, ok := z.Tags.Lookup(tag)
			if !ok {
				log.Printf(`WARN tag "%v" has no associated interval [Delete]\n`, tag.Name())
				continue // non-fatal error, ignore
			}
			z.maybeAdjustTagIntervalForDelete(tag, interval, fromTo)
		}
	}

	rowNumBefore := len(z.Rows)

	if fromTo.Start.Line == fromTo.End.Line && fromTo.Start.Column == z.LastColumn(fromTo.Start.Line) {
		// SPECIAL CASE: The very last char of a line is removed, which must be a line ending delimiter.
		// If there is a next line, it is appended to this line, including its delimiter.
		z.Rows[fromTo.Start.Line] = slices.Delete(z.Rows[fromTo.Start.Line], fromTo.Start.Column,
			fromTo.Start.Column+1)
		if z.LastLine() > fromTo.Start.Line {
			z.Rows[fromTo.Start.Line] = append(z.Rows[fromTo.Start.Line], slices.Clone(z.Rows[fromTo.Start.Line+1])...)
			z.Rows = slices.Delete(z.Rows, fromTo.Start.Line+1, fromTo.Start.Line+2)
		}
		// Adjust the caret for this case.
		if z.caretPos.Line == fromTo.Start.Line+1 {
			z.SetCaret(fromTo.Start)
		}
	} else {
		// NORMAL CASE: Delete the range from fromTo.Start.Line to fromTo.End.Line in the Rows.
		// Whatever is behind this range on the end line is added to the start line.
		underflow := slices.Clone(z.Rows[fromTo.End.Line][fromTo.End.Column+1:])
		z.Rows[fromTo.Start.Line] = z.Rows[fromTo.Start.Line][:fromTo.Start.Column]
		z.Rows[fromTo.Start.Line] = append(z.Rows[fromTo.Start.Line], underflow...)
		z.Rows = slices.Delete(z.Rows, fromTo.Start.Line+1, fromTo.End.Line+1)
		// Adjust the caret as needed for this case.
		if CmpPos(fromTo.End, z.caretPos) < 0 {
			if fromTo.End.Line == z.caretPos.Line {
				z.SetCaret(CharPos{Line: z.caretPos.Line - (fromTo.End.Line - fromTo.Start.Line),
					Column: fromTo.Start.Column + (z.caretPos.Column - fromTo.End.Column) - 1})
			} else {
				z.SetCaret(CharPos{Line: z.caretPos.Line - (fromTo.End.Line - fromTo.Start.Line),
					Column: z.caretPos.Column})
			}
		} else if CmpPos(fromTo.Start, z.caretPos) <= 0 {
			z.SetCaret(fromTo.Start)
		}
	}

	// The first line might be empty now. If so, we add an appropriate line ending.
	if len(z.Rows[fromTo.Start.Line]) == 0 {
		if z.Config.SoftWrap {
			z.Rows[fromTo.Start.Line] = append(z.Rows[fromTo.Start.Line], z.Config.SoftLF)
		} else {
			z.Rows[fromTo.Start.Line] = append(z.Rows[fromTo.Start.Line], z.Config.HardLF)
		}
	}

	// Now we reflow with word wrap like in Insert.
	paraStart := z.FindParagraphStart(fromTo.Start.Line, z.Config.HardLF)
	paraEnd := z.FindParagraphEnd(fromTo.Start.Line, z.Config.HardLF)
	rows := make([][]rune, paraEnd-paraStart+1)
	for i := range rows {
		rows[i] = z.Rows[i+paraStart]
	}
	tags, ok = z.Tags.LookupRange(z.ToEnd(fromTo.Start))
	newCursorRow := z.caretPos.Line
	newCursorCol := z.caretPos.Column
	rows, newCursorRow, newCursorCol = z.WordWrapRows(rows, z.Columns, z.Config.SoftWrap, z.Config.HardLF,
		z.Config.SoftLF, newCursorRow-paraStart, newCursorCol, paraStart, tags, fromTo.Start)

	// Check if we need to delete rows.
	if len(rows) < paraEnd-paraStart+1 {
		z.Rows = slices.Delete(z.Rows, paraStart+len(rows), paraEnd+1)
	}

	// Check if we need to insert additional rows.
	if len(rows) > paraEnd-paraStart+1 {
		newRows := makeEmptyRows(len(rows) - (paraEnd - paraStart + 1))
		z.Rows = slices.Insert(z.Rows, paraEnd+1, newRows...)
	}
	for i := range rows {
		z.Rows[i+paraStart] = rows[i]
	}
	lineDelta := rowNumBefore - len(z.Rows)
	z.adjustTagLines(tags, -lineDelta, fromTo.Start)
	z.SetCaret(CharPos{Line: newCursorRow + paraStart, Column: min(newCursorCol, len(z.Rows[newCursorRow+paraStart])-1)})
	z.Refresh()
}

// ToEnd returns the char interval from the given position to the last char of the buffer.
func (z *Editor) ToEnd(start CharPos) CharInterval {
	return CharInterval{Start: start, End: z.LastPos()}
}

// LastPos returns the last char position in the buffer.
func (z *Editor) LastPos() CharPos {
	return CharPos{Line: len(z.Rows) - 1, Column: len(z.Rows[len(z.Rows)-1]) - 1}
}

// PrevPos returns the previous char position in the grid and true, or 0, 0 and false if at home position.
func (z *Editor) PrevPos(pos CharPos) (CharPos, bool) {
	if pos.Line <= 0 && pos.Column <= 0 {
		return CharPos{Line: 0, Column: 0}, false
	}
	if pos.Column == 0 {
		return CharPos{Line: pos.Line - 1, Column: len(z.Rows[pos.Line-1]) - 1}, true
	}
	return CharPos{Line: pos.Line, Column: pos.Column - 1}, true
}

// maybeAdjustTagIntervalForDelete adjusts the given interval based on deleting fromTo. This has 8 cases
// and some of them require knowing the lengths of the lines. See Delete for usage of this method.
// No word wrapping is assumed since this is handled separately.
// Cases to consider:
//
//	Case 1: A is within B.
//	Case 2: A overlaps into B from the left.
//	Case 3: B is within A.
//	Case 4: A is strictly after B.
//	Case 5: A is strictly before B and ends on the line B starts.
//	Case 6: A is strictly before B and ends before the line B starts.
//	Case 7: A overlaps into B from the right.
func (z *Editor) maybeAdjustTagIntervalForDelete(tag Tag, interval, fromTo CharInterval) {
	// Case 4: fromTo is strictly after interval => Do nothing.
	if CmpPos(fromTo.Start, interval.End) > 0 {
		log.Println("CASE 4")
		return
	}
	lineDelta := fromTo.End.Line - fromTo.Start.Line
	lineDelta = -lineDelta
	columnDelta := fromTo.End.Column
	if fromTo.Start.Line == fromTo.End.Line {
		columnDelta -= fromTo.Start.Column
		columnDelta++
	}
	columnDelta = -columnDelta
	log.Println(columnDelta)
	if CmpPos(fromTo.End, interval.Start) < 0 {
		// Cases 5 and 6.
		if fromTo.End.Line < interval.Start.Line {
			// Case 6: We shift the interval by lineDelta, no other changes needed.
			log.Println("CASE 6")
			newInterval := CharInterval{Start: CharPos{Line: interval.Start.Line + lineDelta, Column: interval.Start.Column},
				End: CharPos{Line: interval.End.Line + lineDelta, Column: interval.End.Column}}
			z.Tags.Upsert(tag, newInterval)
			return
		}
		// Case 5: We shift the interval by lineDelta but also have to shift the start column.
		log.Println("CASE 5")
		var newInterval CharInterval
		diff1 := interval.Start.Column - fromTo.End.Column
		if interval.Start.Line == interval.End.Line {
			// Special case: The interval ends on the same line, so the end has to be adjusted, too.
			newInterval = CharInterval{Start: CharPos{Line: fromTo.Start.Line, Column: fromTo.Start.Column + diff1 - 1},
				End: CharPos{Line: fromTo.Start.Line,
					Column: fromTo.Start.Column + (interval.End.Column - interval.Start.Column) + diff1 - 1}}
		} else {
			newInterval = CharInterval{Start: CharPos{Line: fromTo.Start.Line, Column: fromTo.Start.Column + diff1 - 1},
				End: CharPos{Line: fromTo.Start.Line + (interval.End.Line - interval.Start.Line), Column: interval.End.Column}}
		}
		z.Tags.Upsert(tag, newInterval)
		return
	}
	if CmpPos(fromTo.Start, interval.Start) <= 0 && CmpPos(fromTo.End, interval.End) >= 0 {
		// Case 3: We can delete the tag, because the entire interval is being deleted.
		log.Println("CASE 3")
		z.Tags.Delete(tag)
		return
	}
	if CmpPos(fromTo.Start, interval.Start) >= 0 && CmpPos(fromTo.End, interval.End) <= 0 {
		// Case 1: The deletion interval is within the interval. (Note: Exact equality already handled above.)
		// Only the end column has to be adjusted. Whatever is deleted in the start line does not affect the interval.
		log.Println("CASE 1")
		if fromTo.End.Line != interval.End.Line {
			columnDelta = 0
		}
		lfRemoved := 0
		if fromTo.Start.Line == fromTo.End.Line && fromTo.Start.Column == z.LastColumn(fromTo.Start.Line) {
			lfRemoved = -1
		}
		newInterval := CharInterval{Start: CharPos{Line: interval.Start.Line, Column: interval.Start.Column},
			End: CharPos{Line: interval.End.Line - fromTo.Lines() + 1 + lfRemoved,
				Column: interval.End.Column + columnDelta}}
		z.Tags.Upsert(tag, newInterval)
		return
	}
	if CmpPos(fromTo.Start, interval.Start) < 0 {
		// Case 2: The new interval starts at fromTo.Start. We may need to adjust the end column and need to subtract lineDelta.
		log.Println("CASE 2")
		if fromTo.End.Line != interval.End.Line {
			columnDelta = 0
		}
		newInterval := CharInterval{Start: fromTo.Start,
			End: CharPos{Line: interval.End.Line + lineDelta, Column: interval.End.Column + columnDelta}}
		z.Tags.Upsert(tag, newInterval)
		return
	}
	if CmpPos(fromTo.Start, interval.Start) >= 0 && CmpPos(fromTo.End, interval.End) > 0 {
		// Case 7: Adjust by lineDelta and the new column will be fromTo. Start.
		log.Println("CASE 7")
		newInterval := CharInterval{Start: interval.Start, End: fromTo.Start}
		z.Tags.Upsert(tag, newInterval)
		return
	}
	log.Printf("zedit.Editor.Delete: An interval adjustment was left unhandled, which should never occur. fromTo: %v, interval to adjust: %v\n", fromTo, interval)
}

// NextPos returns the next char position in the grid and true, or the last position and false if there is no more.
func (z *Editor) NextPos(pos CharPos) (CharPos, bool) {
	if pos.Line >= len(z.Rows)-1 && pos.Column >= len(z.Rows[pos.Line])-1 {
		return CharPos{Line: len(z.Rows) - 1, Column: len(z.Rows[len(z.Rows)-1]) - 1}, false
	}
	if pos.Column >= len(z.Rows[pos.Line])-1 {
		return CharPos{Line: pos.Line + 1, Column: 0}, true
	}
	return CharPos{Line: pos.Line, Column: pos.Column + 1}, true
}

// Backspace deletes the character left of the caret, if there is one.
func (z *Editor) Backspace() {
	to := z.caretPos
	from, changed := z.PrevPos(to)

	if !changed {
		return
	}
	z.Delete(CharInterval{Start: from, End: from})
}

// Delete1 deletes the character under the caret or the selection, if there is one.
func (z *Editor) Delete1() {
	from := z.caretPos
	z.Delete(CharInterval{Start: from, End: from}) // char intervals are inclusive on both start and end
	return
}

// Return implements the return key behavior, which creates a new line and advances the caret accordingly.
func (z *Editor) Return() {
	pos := z.caretPos
	if pos.Column == 0 {
		z.Rows = slices.Insert(z.Rows, pos.Line, []rune{z.Config.HardLF})
		z.MoveCaret(CaretDown)
		z.Refresh()
		return
	}
	buff := z.Rows[pos.Line][pos.Column:]
	z.Rows[pos.Line] = z.Rows[pos.Line][:pos.Column]
	z.Rows = slices.Insert(z.Rows, pos.Line+1, slices.Clone(buff))
	z.Refresh()
	z.MoveCaret(CaretRight)
}

// STYLES

// maybeStyleRange styles the given char interval by style insofar as it is within
// the visible range of the underlying TextGrid (otherwise, nothing is done).
func (z *Editor) maybeStyleRange(tag Tag, interval CharInterval, styler TagStyleFunc, drawFullLine bool) {
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
				z.grid.Rows[i].Cells[j] = styler(tag, z.grid.Rows[i].Cells[j])
			}
		}
	}
}

func (z *Editor) lineNumberLen() int {
	s := strconv.Itoa(len(z.Rows))
	return len(s)
}

func (s *Editor) CreateRenderer() fyne.WidgetRenderer {
	return &zgridRenderer{zgrid: s}
}

type zgridRenderer struct {
	zgrid *Editor
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

func substring(s string, start int, end int) string {
	start_str_idx := 0
	i := 0
	for j := range s {
		if i == start {
			start_str_idx = j
		}
		if i == end {
			return s[start_str_idx:j]
		}
		i++
	}
	return s[start_str_idx:]
}

// SafePositiveValue returns a sanitized integer that is 0 or larger
// and no larger than the given maximum value (inclusive).
func SafePositiveValue(value int, maximum int) int {
	return min(max(0, value), maximum)
}

// WORD WRAPPING
// Ad hoc struct for holding text grid cells plus hosuekeeping info.
type xCell struct {
	Rune         rune
	Row          *[]rune
	IsCursorCell bool
	tags         []xTag
}

// Ad hoc struct for holding a tag and whether we record the start of the tag's interval
// or the end. We use this to record start and end of tags directly in xCell.
type xTag struct {
	tag     Tag
	isStart bool
}

// WordWrapRows word wraps a number of rows, making sure soft line breaks are adjusted
// and removed accordingly. The number of rows returned may be larger than the number of rows
// provided as an argument. The position of the original cursor row and column is returned.
func (z *Editor) WordWrapRows(rows [][]rune, wrapCol int,
	softWrap bool, hardLF, softLF rune, cursorRow, cursorCol, startRow int,
	tags []Tag, pos CharPos) ([][]rune, int, int) {
	para := make([]xCell, 0)
	// 1. push all characters into one array of extended cells
	// but ignore line breaks
	cursorToNext := false
	for i := range rows {
		for j, c := range rows[i] {
			isCursor := false
			if (i == cursorRow && j == cursorCol) || cursorToNext {
				isCursor = true
				cursorToNext = false
			}
			if (c == hardLF && j == len(rows[i])-1) || c == softLF {
				if i == cursorRow && j == cursorCol {
					cursorToNext = true // delete LF but make sure cursor will be on next char
				}
			} else {
				var tg []xTag
				line := startRow + i
				for _, tag := range tags {
					if tag == nil {
						continue
					}
					interval, found := z.Tags.Lookup(tag)
					if !found {
						continue
					}
					isStart := CmpPos(interval.Start, CharPos{Line: line, Column: j}) == 0
					isEnd := CmpPos(interval.End, CharPos{Line: line, Column: j}) == 0
					if !isStart && !isEnd {
						continue
					}
					if tg == nil {
						tg = make([]xTag, 0)
					}
					if isStart {
						tg = append(tg, xTag{tag: tag, isStart: true})
					}
					if isEnd {
						tg = append(tg, xTag{tag: tag, isStart: false})
					}
				}
				para = append(para, xCell{Rune: c, Row: &rows[i], IsCursorCell: isCursor, tags: tg})
			}
		}
	}
	// 2. now word break the paragraph and push into a result array
	// adding soft line breaks, and the final hard line break
	result := make([][]rune, 0)
	lastSpc := 0
	line := make([]xCell, 0, wrapCol+1)
	var overflow []xCell
	col := 0
	newCol := cursorCol
	newRow := 0
	var currentRow []rune
	var handled bool
	lpos := 0
	lineIdx := 0
	for i := range para {
		handled = false
		c := para[i]
		lpos++
		line = append(line, c)
		if unicode.IsSpace(c.Rune) {
			lastSpc = lpos // space position + 1 because of lpos++
		}
		if lpos >= wrapCol {
			cutPos := lpos
			if lastSpc > 0 {
				cutPos = min(lpos, lastSpc)
			}
			if cutPos >= wrapCol/2 && cutPos < len(line) {
				overflow = make([]xCell, 0, len(line)-cutPos)
				overflow = append(overflow, line[cutPos:]...)
				line = line[:cutPos]
			}
			currentRow, col = xCellsToRow(line)

			// adjust the tags if necessary
			z.adjustTags(line, startRow, lineIdx)

			if col >= 0 {
				newCol = col
			}
			result = append(result, currentRow)
			if cellsContainCursor(line) {
				newRow = lineIdx
			}
			if overflow != nil && len(overflow) > 0 {
				line = overflow
				if cellsContainCursor(line) {
					newCol = len(line) - 1
				}
				overflow = nil
				lpos = len(line)
			} else {
				line = make([]xCell, 0, wrapCol)
				handled = true
				lpos = 0
			}
			lastSpc = 0
			lineIdx++
		}
	}
	if !handled {
		currentRow, col = xCellsToRow(line)
		z.adjustTags(line, startRow, lineIdx)
		if col >= 0 {
			newCol = col
		}
		result = append(result, currentRow)
		if cellsContainCursor(line) {
			newRow = lineIdx
		}
	}
	for i := range result {
		if softWrap {
			result[i] = append(result[i], softLF)
		} else {
			result[i] = append(result[i], hardLF)
		}
	}
	k := len(result) - 1
	n := len(result[k]) - 1
	result[k][n] = hardLF
	// The following can *only* happen if the cursor was at the very last LF,
	// which had been deleted; see Step 1 above. So we set it to the pragraph end.
	if cursorToNext {
		newRow = k
		newCol = n
	}
	return result, newRow, newCol
}

// adjustTags adjusts the intervals of tags recorded in xCell if necessary.
// This has bad complexity but note we only recorded start and end positions.
func (z *Editor) adjustTags(line []xCell, startRow, lineIdx int) {
outer:
	for j, c := range line {
		if c.tags == nil {
			continue outer
		}
	inner:
		for _, xtag := range c.tags {
			interval, found := z.Tags.Lookup(xtag.tag)
			if !found {
				continue inner
			}
			if xtag.isStart {
				z.Tags.Upsert(xtag.tag, CharInterval{Start: CharPos{Line: startRow + lineIdx, Column: j},
					End: interval.End})
			} else {
				z.Tags.Upsert(xtag.tag, CharInterval{Start: interval.Start,
					End: CharPos{Line: startRow + lineIdx, Column: j}})
			}
		}
	}
}

func xCellsToRow(cells []xCell) ([]rune, int) {
	if len(cells) == 0 {
		return make([]rune, 0), -1
	}
	result := make([]rune, len(cells))
	col := -1
	for i, c := range cells {
		result[i] = c.Rune
		if c.IsCursorCell {
			col = i
		}
	}
	return result, col
}

func cellsContainCursor(cells []xCell) bool {
	for _, c := range cells {
		if c.IsCursorCell {
			return true
		}
	}
	return false
}

// inSelectionRange is true if row, col is within the range startRow, startCol to endRow, endCol,
// false otherwise.
func inSelectionRange(startRow, startCol, endRow, endCol, row, col int) bool {
	return (row == startRow && col >= startCol) || (row == endRow && col <= endCol) || (row > startRow && row < endRow)
}

// makeEmptyRows creates n text rows initialized to hold a single glyph '\n'
func makeEmptyRows(n int) [][]rune {
	rows := make([][]rune, n)
	for i := range rows {
		rows[i] = make([]rune, 0)
	}
	return rows
}

// IsParen returns true if the rune is a left paren.
func IsLeftParen(c rune) bool {
	switch c {
	case '(', '[', '{':
		return true
	default:
		return false
	}
}

// IsRightParen returns true if the rune is a right paren.
func IsRightParen(c rune) bool {
	switch c {
	case ')', ']', '}':
		return true
	default:
		return false
	}
}

// IsQuotationMark returns true if the rune is a quotation mark, which behaves similar to a paren
// but is symmetric (i.e., opening and closing marks are identical).
func IsQuotationMark(c rune) bool {
	switch c {
	case '"', '\'':
		return true
	default:
		return false
	}
}

// MakeKeyboardShortcutKey makes a lookup key for a fyne.KeyboardShortcut that is equal
// for any two shortcuts with the same key and modifier. (The shortcut name does not have this
// property.)
func GetKeyboardShortcutKey(s fyne.KeyboardShortcut) string {
	return fmt.Sprintf("%v:%v", s.Key(), s.Mod())
}
