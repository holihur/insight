package callgraph

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// parseCacheVersion must be bumped whenever tree-sitter extraction output
// changes shape, so old caches are ignored instead of misread.
const parseCacheVersion = 1

const (
	parseCacheMaxEntries = 50000
	parseCacheMaxBytes   = 96 << 20
)

// cachedParse is the tree-sitter result for one file content hash.
type cachedParse struct {
	Version int      `json:"v"`
	Lang    string   `json:"lang"`
	Defs    []tsDef  `json:"defs,omitempty"`
	Calls   []tsCall `json:"calls,omitempty"`
	Used    int64    `json:"used,omitempty"`
}

// parseCache is a content-addressed cache of parsed files. Because the key is
// the file hash, entries stay valid across commits and re-clones: only files
// whose bytes changed are parsed again, which is what makes indexing
// incremental.
type parseCache struct {
	mu      sync.Mutex
	entries map[string]*cachedParse
	hits    int
	misses  int
	dirty   bool
	now     int64
}

func newParseCache() *parseCache {
	return &parseCache{entries: map[string]*cachedParse{}}
}

func (c *parseCache) beginBuild() {
	c.mu.Lock()
	c.hits, c.misses = 0, 0
	c.now = time.Now().UnixNano()
	c.mu.Unlock()
}

func (c *parseCache) stats() (hits, misses int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits, c.misses
}

func (c *parseCache) key(lang string, content []byte) string {
	sum := sha1.Sum(content)
	return lang + ":" + hex.EncodeToString(sum[:])
}

func (c *parseCache) get(key string) (*cachedParse, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || entry.Version != parseCacheVersion {
		c.misses++
		return nil, false
	}
	c.hits++
	entry.Used = c.now
	return entry, true
}

func (c *parseCache) put(key string, entry *cachedParse) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry.Version = parseCacheVersion
	entry.Used = c.now
	c.entries[key] = entry
	c.dirty = true
}

func (c *parseCache) takeDirty() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	dirty := c.dirty
	c.dirty = false
	return dirty
}

type parseCacheFile struct {
	Version int                     `json:"version"`
	Entries map[string]*cachedParse `json:"entries"`
}

func loadParseCache(path string) *parseCache {
	cache := newParseCache()
	data, err := os.ReadFile(path)
	if err != nil {
		return cache
	}
	var file parseCacheFile
	if err := json.Unmarshal(data, &file); err != nil || file.Version != parseCacheVersion {
		return cache
	}
	for key, entry := range file.Entries {
		if entry == nil || entry.Version != parseCacheVersion {
			continue
		}
		cache.entries[key] = entry
	}
	return cache
}

func (c *parseCache) save(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if len(c.entries) > parseCacheMaxEntries || c.estimateBytesLocked() > parseCacheMaxBytes {
		c.pruneLocked()
	}
	payload, err := json.Marshal(parseCacheFile{Version: parseCacheVersion, Entries: c.entries})
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (c *parseCache) estimateBytesLocked() int64 {
	// Rough estimate: 160 bytes per definition/call plus keys.
	var total int64
	for key, entry := range c.entries {
		total += int64(len(key)) + int64(len(entry.Defs)+len(entry.Calls))*160
	}
	return total
}

func (c *parseCache) pruneLocked() {
	type item struct {
		key  string
		used int64
	}
	items := make([]item, 0, len(c.entries))
	for key, entry := range c.entries {
		items = append(items, item{key: key, used: entry.Used})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].used > items[j].used })
	keep := parseCacheMaxEntries
	if keep > len(items) {
		keep = len(items)
	}
	kept := make(map[string]*cachedParse, keep)
	for _, it := range items[:keep] {
		kept[it.key] = c.entries[it.key]
	}
	c.entries = kept
}
