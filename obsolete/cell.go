package zedit

import (
	"sync"

	"github.com/bits-and-blooms/bitset"
)

// EditorCell represents a rune and arbitrary many flags which are associated with tags in a Tags
// container. This is a memory-efficient storage of tags over regions, though not as memory-efficient as
// intervals. A previous implementation used an interval tree and a different tag system but it turned out to
// be too complex for handling editing operation.
type EditorCell struct {
	Rune  rune
	Flags bitset.BitSet
}

// EditorFlag is a numerical key associated with a tag.
type EditorFlag uint

// TagMap is a concurrent store with flags as key and tags as values.
type TagMap struct {
	tags  map[EditorFlag]Tag
	mutex sync.RWMutex
}

// NewTagMap returns a new, empty Tags container.
func NewTagMap() *TagMap {
	return &TagMap{
		tags: make(map[EditorFlag]Tag),
	}
}

// GetAll returns all tags associated with the given flags. If there is no corresponding mapping, then no tag is
// added.
func (c *TagMap) GetAll(flags []EditorFlag) []Tag {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	tags := make([]Tag, 0, len(flags))
	for _, flag := range flags {
		t, ok := c.tags[flag]
		if !ok {
			continue
		}
		tags = append(tags, t)
	}
	return tags
}

// Get returns a single tag and true for a given flag, false when there is no tag for the flag as key.
func (c *TagMap) Get(flag EditorFlag) (Tag, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	t, ok := c.tags[flag]
	return t, ok
}

// Set associates a flag with a tag in the container.
func (c *TagMap) Set(flag EditorFlag, tag Tag) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.tags[flag] = tag
}

// Remove deletes the tag for the given flag.
func (c *TagMap) Remove(flag EditorFlag) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.tags, flag)
}
