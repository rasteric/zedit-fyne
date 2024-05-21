package zedit

import (
	"encoding/json"
	"log"
	"reflect"
	"slices"
	"strconv"
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
type TagStyleFunc func(tag Tag, c widget.TextGridCell) widget.TextGridCell
type CustomTagUnmarshallerFunc func(typeName string, in []byte) (Tag, error)

// CustomTagUnmarshaller should be set to a function that takes the type name and []byte,
// and returns the appropriate custom Tag based on how MarshalJSON is implemented for that tag.
// The unmarshaller for all tags will automatically dispatch to this function when unmarshalling
// a custom tag implemented by the user of this package. This is a bit safer and more flexible
// than trying to do this automatically using reflection.
var CustomTagUnmarshaller CustomTagUnmarshallerFunc

// Tags are used to store information about the editor text associated with intervals.
// A tag's position is adjusted automatically as the editor text changes.
// Stylers can be associated to multiple tags with the same name.
type Tag interface {
	Name() string                    // return the tag's name, which is used for stylers
	Index() int                      // return the tag's new index, a serial number for tags with the same name
	Clone(newIndex int) Tag          // clone the tag, giving it a new index
	Callback() TagFunc               // called when TagEvents happen
	SetCallback(cb TagFunc)          // set the callback function
	UserData() any                   // optional payload
	SetUserData(data any)            // set optional payload
	MarshalJSON() ([]byte, error)    // for serialization
	UnmarshalJSON(data []byte) error // for deserialization
}

// StandardTag is the default implementation of a Tag.
type StandardTag struct {
	name    string
	index   int
	payload any
	cb      TagFunc
}

// tagData is only used for serialization, write your own custom marshaler
// for JSON if you implement a different tag
type tagData struct {
	Name  string
	Index json.Number
}

// TagSyler styles tags with the given name.
type TagStyler struct {
	TagName      string
	StyleFunc    TagStyleFunc
	DrawFullLine bool
}

// TagWithInterval stores a tag and its accompanying interval.
type TagWithInterval struct {
	Tag      Tag
	Interval CharInterval
}

type typedTagWithInterval struct {
	Type     string
	Tag      Tag
	Interval CharInterval
}

func (t TagWithInterval) MarshalJSON() ([]byte, error) {
	var s string
	switch t.Tag.(type) {
	case *StandardTag:
		s = "StandardTag"
	default:
		s = reflect.TypeOf(t.Tag).String()
	}
	b, err := json.Marshal(typedTagWithInterval{
		Type:     s,
		Tag:      t.Tag,
		Interval: t.Interval,
	})
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (t *TagWithInterval) UnmarshalJSON(in []byte) error {
	var data struct {
		Type     string
		Tag      json.RawMessage
		Interval json.RawMessage
	}
	err := json.Unmarshal(in, &data)
	if err != nil {
		return err
	}
	var tag Tag
	switch data.Type {
	case "StandardTag":
		s := &StandardTag{}
		err := json.Unmarshal(data.Tag, s)
		if err != nil {
			return err
		}
		tag = s
	default:
		if CustomTagUnmarshaller == nil {
			log.Printf("failed to read some text data, unsupported custom tag: %v\n", data.Type)
		} else {
			tag, err = CustomTagUnmarshaller(data.Type, data.Tag)
			if err != nil {
				return err
			}
		}
	}
	var interval CharInterval
	err = json.Unmarshal(data.Interval, &interval)
	if err != nil {
		return err
	}
	t.Tag = tag
	t.Interval = interval
	return nil
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

func (s StandardTag) MarshalJSON() ([]byte, error) {
	tag := tagData{Name: s.Name(), Index: json.Number(strconv.Itoa(s.Index()))}
	return json.Marshal(tag)
}

func (s *StandardTag) UnmarshalJSON(data []byte) error {
	tag := tagData{}
	err := json.Unmarshal(data, &tag)
	if err != nil {
		return err
	}
	s.name = tag.Name
	idx, err := tag.Index.Int64()
	if err != nil {
		return err
	}
	s.index = int(idx)
	return nil
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

func (c *TagContainer) AllTags() []TagWithInterval {
	all := make([]TagWithInterval, 0)
	for k, v := range c.tags {
		all = append(all, TagWithInterval{Tag: k, Interval: v})
	}
	return all
}

func (c *TagContainer) SetAllTags(tags []TagWithInterval) {
	c.Clear()
	for _, tag := range tags {
		if tag.Tag == nil {
			continue
		}
		c.Add(tag.Interval, tag.Tag)
	}
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
