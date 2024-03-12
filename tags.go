package zedit

import (
	"slices"
	"sync"

	"fyne.io/fyne/v2/widget"
	"github.com/rdleal/intervalst/interval"
)

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

func NewTag(name string) *SimpleTag {
	return &SimpleTag{name: name}
}

func (s *SimpleTag) Name() string {
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
		t.lookup.Delete(interval.Start, interval.End)
		for i := range tags {
			if tags[i].Name() == tag.Name() {
				delete(t.names, tag.Name())
				tags[i] = tags[len(tags)-1]
				tags = tags[:len(tags)-1]
				break
			}
		}
		t.lookup.Insert(interval.Start, interval.End, tags...)
		t.mutex.Unlock()
	}
	return ok
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
