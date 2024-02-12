package zedit

import (
	"slices"
	"sync"

	"fyne.io/fyne/v2/widget"
	"github.com/rdleal/intervalst/interval"
)

// CharPos represents a character position in the grid.
// If IsLineNumber is true, then the position is in the line
// number display, Line contains the line number, and Column is 0.
// Otherwise, Line and Column contain the line, column pair
// of the grid closest to the position.
type CharPos struct {
	Line         int
	Column       int
	IsLineNumber bool
}

// CmpPos lexicographically compares to char positions and
// returns -1 if a is before b, 0 if a and b are equal positions,
// and 1 if b is before a. This is used for interval operation.
func CmpPos(a, b CharPos) int {
	if a.Line < b.Line {
		return -1
	}
	if a.IsLineNumber {
		if a.Line == b.Line {
			return 0
		}
		return 1
	}
	if a.Line > b.Line {
		return 1
	}
	if a.Column < b.Column {
		return -1
	}
	if a.Column == b.Column {
		return 0
	}
	return 1
}

func MaxPos(a, b CharPos) CharPos {
	n := CmpPos(a, b)
	if n == -1 {
		return b
	}
	return a
}

func MinPos(a, b CharPos) CharPos {
	n := CmpPos(a, b)
	if n <= 0 {
		return a
	}
	return b
}

type CharInterval struct {
	Start CharPos
	End   CharPos
}

// OutsideOf returns true if c1 is outside of c2.
func (c1 CharInterval) OutsideOf(c2 CharInterval) bool {
	return CmpPos(c1.End, c2.Start) == -1 || CmpPos(c1.Start, c2.End) == 1
}

// Contains returns true if the char interval contains the given position, false otherwise.
func (c CharInterval) Contains(pos CharPos) bool {
	return CmpPos(pos, c.Start) >= 0 && CmpPos(pos, c.End) <= 0
}

// MaybeSwap compares the start and the end, and if the end is before
// the start returns the interval where the end is the start and the start is the end.
// The function returns the unchanged interval otherwise.
func (c CharInterval) MaybeSwap() CharInterval {
	if CmpPos(c.Start, c.End) > 0 {
		return CharInterval{Start: c.End, End: c.Start}
	}
	return c
}

type Tag interface {
	Name() string
}

type SimpleTag struct {
	name string
}

type TagStyleFunc func(c widget.TextGridCell) widget.TextGridCell
type TagLineStyleFunc func(style widget.TextGridStyle) widget.TextGridStyle

type TagStyler struct {
	Tag          Tag
	StyleFunc    TagStyleFunc
	DrawFullLine bool
}

type LineStyler struct {
	Tag           Tag
	LineStyleFunc TagLineStyleFunc
}

func NewTag(name string) SimpleTag {
	return SimpleTag{name: name}
}

func (s SimpleTag) Name() string {
	return s.name
}

type TagContainer struct {
	names       map[string]CharInterval
	lookup      *interval.MultiValueSearchTree[Tag, CharPos]
	stylers     []TagStyler
	lineStylers []LineStyler
	mutex       sync.Mutex
}

func NewTagContainer() *TagContainer {
	tags := TagContainer{}
	tags.names = make(map[string]CharInterval)
	tags.lookup = interval.NewMultiValueSearchTreeWithOptions[Tag, CharPos](CmpPos, interval.TreeWithIntervalPoint())
	return &tags
}

func (t *TagContainer) LookupRange(interval CharInterval) ([]Tag, bool) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.lookup.AnyIntersection(interval.Start, interval.End)
}

func (t *TagContainer) Lookup(tag Tag) (CharInterval, bool) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	interval, ok := t.names[tag.Name()]
	return interval, ok
}

func (t *TagContainer) Add(interval CharInterval, tags ...Tag) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	for _, tag := range tags {
		t.names[tag.Name()] = interval
	}
	t.lookup.Insert(interval.Start, interval.End, tags...)
}

func (t *TagContainer) Delete(tag Tag) bool {
	t.mutex.Lock()
	interval, ok := t.names[tag.Name()]
	t.mutex.Unlock()
	if !ok {
		return false
	}
	tags, ok := t.LookupRange(interval)
	if ok {
		t.mutex.Lock()
		for i := range tags {
			if tags[i].Name() == tag.Name() {
				tags[i] = tags[len(tags)-1]
				tags = tags[:len(tags)-1]
				break
			}
		}
		t.mutex.Unlock()
		t.lookup.Upsert(interval.Start, interval.End, tags...)
	}
	t.mutex.Lock()
	delete(t.names, tag.Name())
	t.mutex.Unlock()
	return true
}

func (t *TagContainer) Upsert(tag Tag, interval CharInterval) {
	if _, ok := t.names[tag.Name()]; ok {
		t.Delete(tag)
	}
	t.Add(interval, tag)
}

func (t *TagContainer) AddStyler(styler TagStyler) {
	if t.stylers == nil {
		t.stylers = make([]TagStyler, 1)
		t.stylers[0] = styler
		return
	}
	t.stylers = append(t.stylers, styler)
}

func (t *TagContainer) RemoveStyler(tag Tag) {
	if t.stylers == nil {
		return
	}
	t.stylers = slices.DeleteFunc(t.stylers, func(styler TagStyler) bool {
		return styler.Tag.Name() == tag.Name()
	})
}

func (t *TagContainer) AddLineStyler(styler LineStyler) {
	if t.lineStylers == nil {
		t.lineStylers = make([]LineStyler, 1)
		t.lineStylers[0] = styler
		return
	}
	t.lineStylers = append(t.lineStylers, styler)
}

func (t *TagContainer) RemoveLineStyler(tag Tag) {
	if t.lineStylers == nil {
		return
	}
	t.lineStylers = slices.DeleteFunc(t.lineStylers, func(styler LineStyler) bool {
		return styler.Tag.Name() == tag.Name()
	})
}

type CellBuffer struct {
	buffs map[string][]widget.TextGridCell
}

func NewCellBuffer() *CellBuffer {
	b := CellBuffer{}
	b.buffs = make(map[string][]widget.TextGridCell)
	return &b
}

func (b *CellBuffer) AppendCell(tag Tag, cell widget.TextGridCell) {
	cells, ok := b.buffs[tag.Name()]
	if !ok {
		cells = make([]widget.TextGridCell, 0)
	}
	cells = append(cells, cell)
	b.buffs[tag.Name()] = cells
}

func (b *CellBuffer) GetCell(tag Tag, i int) (widget.TextGridCell, bool) {
	cells, ok := b.buffs[tag.Name()]
	if !ok {
		return widget.TextGridCell{}, false
	}
	if i > len(cells)-1 || i < 0 {
		return widget.TextGridCell{}, false
	}
	return cells[i], true
}

func (b *CellBuffer) RemoveCells(tag Tag) {
	delete(b.buffs, tag.Name())
}
