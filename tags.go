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
	UserData() any          // optional payload
	SetUserData(data any)   // set optional payload
}

// StandardTag is the default implementation of a Tag.
type StandardTag struct {
	name    string
	index   int
	cb      TagFunc
	payload any
}

type TagStyleFunc func(tag Tag, c widget.TextGridCell) widget.TextGridCell

type TagStyler struct {
	TagName      string
	StyleFunc    TagStyleFunc
	DrawFullLine bool
}

func NewTag(name string) *StandardTag {
	return &StandardTag{name: name}
}

func NewTagWithUserData(name string, index int, userData any) *StandardTag {
	return &StandardTag{name: name, index: index, payload: userData}
}

func (s *StandardTag) Name() string {
	return s.name
}

func (s *StandardTag) Index() int {
	return s.index
}

func (s *StandardTag) Callback() TagFunc {
	return s.cb
}

func (s *StandardTag) SetCallback(cb TagFunc) {
	s.cb = cb
}

func (s *StandardTag) Clone(newIndex int) Tag {
	return &StandardTag{name: s.name, index: newIndex, cb: s.cb}
}

func (s *StandardTag) UserData() any {
	return s.payload
}

func (s *StandardTag) SetUserData(data any) {
	s.payload = data
}

// TagContainer is a container for holding tags and associating them with char intervals. The data structure
// is generally threadsafe but some methods can have race conditions and are documented as such.
type TagContainer struct {
	tags   map[Tag]CharInterval
	lookup *interval.MultiValueSearchTree[Tag, CharPos]
	names  map[string]*orderedset.OrderedSet[Tag]
	mutex  sync.Mutex
}

// NewTagContainer returns a new empty tag container.
func NewTagContainer() *TagContainer {
	c := TagContainer{}
	c.tags = make(map[Tag]CharInterval)
	c.names = make(map[string]*orderedset.OrderedSet[Tag])
	c.lookup = interval.NewMultiValueSearchTreeWithOptions[Tag, CharPos](CmpPos, interval.TreeWithIntervalPoint())
	return &c
}

// Clear deletes all tags in the container but retains stylers.
func (t *TagContainer) Clear() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	clear(t.tags)
	clear(t.names)
	t.lookup = interval.NewMultiValueSearchTreeWithOptions[Tag, CharPos](CmpPos, interval.TreeWithIntervalPoint())
}

// LookupRange returns the tags intersecting with the given char interval.
func (t *TagContainer) LookupRange(interval CharInterval) ([]Tag, bool) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.lookup.AllIntersections(interval.Start, interval.End)
}

// Lookup returns the char interval associated with the given tag.
func (t *TagContainer) Lookup(tag Tag) (CharInterval, bool) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	interval, ok := t.tags[tag]
	return interval, ok
}

// Add adds a number of tags and associates them with the given interval.
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

// Delete deletes the given tag, returns true if the tag was deleted, false if there was no such tag.
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

// DeleteByName deletes all tags with the given name. This method does not guarantee that
// tags that are added concurrently while it is executed are deleted. They may or may not be deleted.
// So, the method is not protected against race conditions. It is generally thread-safe, however.
func (t *TagContainer) DeleteByName(name string) bool {
	set, ok := t.TagsByName(name)
	if !ok {
		return false
	}
	tags := set.Values()
	if tags == nil || len(tags) == 0 {
		return false
	}
	for _, tag := range tags {
		t.Delete(tag)
	}
	return true
}

// Upsert changes the interval associated with the given tag.
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

// StyleContainer holds a number of tag stylers. The data structure is threadsafe.
type StyleContainer struct {
	stylers []TagStyler
	mutex   sync.Mutex
}

func NewStyleContainer() *StyleContainer {
	return &StyleContainer{}
}

// AddStyler adds a new tag styler to the style container.
func (c *StyleContainer) AddStyler(styler TagStyler) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.stylers == nil {
		c.stylers = make([]TagStyler, 1)
		c.stylers[0] = styler
		return
	}
	c.stylers = append(c.stylers, styler)
}

// RemoveStyler removes a tag styler from the container.
func (c *StyleContainer) RemoveStyler(tag Tag) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.stylers == nil {
		return
	}
	c.stylers = slices.DeleteFunc(c.stylers, func(styler TagStyler) bool {
		return styler.TagName == tag.Name()
	})
}

// Stylers returns all tag stylers.
func (c *StyleContainer) Stylers() []TagStyler {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.stylers
}
