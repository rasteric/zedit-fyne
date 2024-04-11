package zedit

import (
	"slices"
	"sync"

	"fyne.io/fyne/v2/widget"
	"github.com/lindell/go-ordered-set/orderedset"
	"github.com/rdleal/intervalst/interval"
)

type TagEvent int

const (
	CaretEnterEvent TagEvent = iota
	CaretLeaveEvent
)

type TagFunc func(evt TagEvent, tag Tag, interval CharInterval)

// Tags are used to store information about the editor text associated with intervals.
// A tag's position is adjusted automatically as the editor text changes.
// Stylers can be associated to multiple tags with the same name.
type Tag interface {
	Name() string           // return the tag's name, which is used for stylers
	Index() int             // return the tag's new index, indicating a serial number for tags with the same name
	Clone(newIndex int) Tag // clone the tag, giving it a new index
	Callback() TagFunc      // called when TagEvents happen
	SetCallback(cb TagFunc) // set the callback function
}

// SimpleTag is the default implementation of a Tag.
type SimpleTag struct {
	name  string
	index int
	cb    TagFunc
}

type TagStyleFunc func(tag Tag, c widget.TextGridCell) widget.TextGridCell

type TagStyler struct {
	TagName      string
	StyleFunc    TagStyleFunc
	DrawFullLine bool
}

func NewTag(name string) *SimpleTag {
	return &SimpleTag{name: name}
}

func (s *SimpleTag) Name() string {
	return s.name
}

func (s *SimpleTag) Index() int {
	return s.index
}

func (s *SimpleTag) Callback() TagFunc {
	return s.cb
}

func (s *SimpleTag) SetCallback(cb TagFunc) {
	s.cb = cb
}

func (s *SimpleTag) Clone(newIndex int) Tag {
	return &SimpleTag{name: s.name, index: newIndex, cb: s.cb}
}

type TagContainer struct {
	tags    map[Tag]CharInterval
	lookup  *interval.MultiValueSearchTree[Tag, CharPos]
	stylers []TagStyler
	names   map[string]*orderedset.OrderedSet[Tag]
	mutex   sync.Mutex
}

func NewTagContainer() *TagContainer {
	c := TagContainer{}
	c.tags = make(map[Tag]CharInterval)
	c.names = make(map[string]*orderedset.OrderedSet[Tag])
	c.lookup = interval.NewMultiValueSearchTreeWithOptions[Tag, CharPos](CmpPos, interval.TreeWithIntervalPoint())
	return &c
}

func (t *TagContainer) LookupRange(interval CharInterval) ([]Tag, bool) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.lookup.AllIntersections(interval.Start, interval.End)
}

func (t *TagContainer) Lookup(tag Tag) (CharInterval, bool) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	interval, ok := t.tags[tag]
	return interval, ok
}

func (t *TagContainer) Add(interval CharInterval, tags ...Tag) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	for _, tag := range tags {
		t.tags[tag] = interval
		if set, ok := t.names[tag.Name()]; ok {
			set.Add(tag)
			t.names[tag.Name()] = set
		} else {
			set := orderedset.New[Tag]()
			set.Add(tag)
			t.names[tag.Name()] = set
		}
	}
	t.lookup.Insert(interval.Start, interval.End, tags...)
}

func (t *TagContainer) Delete(tag Tag) bool {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	interval, ok := t.tags[tag]
	if !ok {
		return false
	}
	delete(t.tags, tag)
	tags, ok := t.lookup.Find(interval.Start, interval.End)
	if ok {
		tags = slices.DeleteFunc(tags, func(tag2 Tag) bool {
			return tag2 == tag
		})
		if set, ok := t.names[tag.Name()]; ok {
			set.Delete(tag)
			t.names[tag.Name()] = set
		}
		if len(tags) > 0 {
			t.lookup.Upsert(interval.Start, interval.End, tags...)
		} else {
			t.lookup.Delete(interval.Start, interval.End)
		}
	}
	return ok
}

func (t *TagContainer) Upsert(tag Tag, interval CharInterval) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	interval2, ok := t.tags[tag]
	if ok {
		tags, ok := t.lookup.Find(interval2.Start, interval2.End)
		if ok {
			tags = slices.DeleteFunc(tags, func(tag2 Tag) bool {
				return tag2 == tag
			})
			if len(tags) > 0 {
				t.lookup.Upsert(interval2.Start, interval2.End, tags...)
			} else {
				t.lookup.Delete(interval2.Start, interval2.End)
			}
		}
	}
	t.tags[tag] = interval
	if set, ok := t.names[tag.Name()]; ok {
		set.Add(tag)
		t.names[tag.Name()] = set
	} else {
		set := orderedset.New[Tag]()
		set.Add(tag)
		t.names[tag.Name()] = set
	}
	t.lookup.Insert(interval.Start, interval.End, tag)
}

// TagsByName returns all tags with the given name. This is used when stylers
// are applied because these work on a by-name basis.
func (t *TagContainer) TagsByName(name string) (*orderedset.OrderedSet[Tag], bool) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	tags, ok := t.names[name]
	return tags, ok
}

// CloneTag clones the given tag with a new index, and registers the tag in the container but without an
// associated interval. If there is no tag in the container, it registers the tag and returns it without cloning it.
func (t *TagContainer) CloneTag(tag Tag) Tag {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	tags, ok := t.names[tag.Name()]
	if ok && tags != nil {
		maxIdx := 0
		loop := tags.Iter()
		for {
			t, ok := loop.Next()
			if !ok {
				break
			}
			if t.Index() > maxIdx {
				maxIdx = t.Index()
			}
		}
		tag = tag.Clone(maxIdx + 1)
	}
	if set, ok := t.names[tag.Name()]; ok {
		set.Add(tag)
		t.names[tag.Name()] = set
	} else {
		set := orderedset.New[Tag]()
		set.Add(tag)
		t.names[tag.Name()] = set
	}
	return tag
}

func (t *TagContainer) AddStyler(styler TagStyler) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.stylers == nil {
		t.stylers = make([]TagStyler, 1)
		t.stylers[0] = styler
		return
	}
	t.stylers = append(t.stylers, styler)
}

func (t *TagContainer) RemoveStyler(tag Tag) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.stylers == nil {
		return
	}
	t.stylers = slices.DeleteFunc(t.stylers, func(styler TagStyler) bool {
		return styler.TagName == tag.Name()
	})
}

func (t *TagContainer) Stylers() []TagStyler {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.stylers
}
