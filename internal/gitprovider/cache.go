// Copyright 2026 — see LICENSE file for terms.
package gitprovider

import (
	"container/list"
	"sync"
	"time"
)

// CacheConfig controls the git object cache.
type CacheConfig struct {
	MaxEntries  int
	MaxMemoryMB int
	TTLSeconds  int
}

// Cache is an in-memory LRU cache for git objects keyed by (repo, objectHash).
type Cache struct {
	mu         sync.Mutex
	entries    map[string]*list.Element
	order      *list.List
	maxEntries int
	ttl        time.Duration
	memUsed    int64
	maxMem     int64 // bytes
}

type cacheEntry struct {
	key     string
	value   any
	size    int64
	expires time.Time
}

// NewCache creates a new LRU cache.
func NewCache(cfg CacheConfig) *Cache {
	maxEntries := cfg.MaxEntries
	if maxEntries <= 0 {
		maxEntries = 10000
	}
	ttl := time.Duration(cfg.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	maxMem := int64(cfg.MaxMemoryMB) * 1024 * 1024
	if maxMem <= 0 {
		maxMem = 256 * 1024 * 1024
	}

	return &Cache{
		entries:    make(map[string]*list.Element),
		order:      list.New(),
		maxEntries: maxEntries,
		ttl:        ttl,
		maxMem:     maxMem,
	}
}

// cacheKey creates a key from repo path and object hash.
func cacheKey(repo, hash string) string {
	return repo + ":" + hash
}

// Get retrieves a cached value. Returns nil if not found or expired.
func (c *Cache) Get(repo, hash string) any {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey(repo, hash)
	elem, ok := c.entries[key]
	if !ok {
		return nil
	}

	entry := elem.Value.(*cacheEntry)
	if time.Now().After(entry.expires) {
		c.removeElement(elem)
		return nil
	}

	c.order.MoveToFront(elem)
	return entry.value
}

// Put adds or updates a cache entry.
func (c *Cache) Put(repo, hash string, value any, size int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey(repo, hash)

	// Update existing entry.
	if elem, ok := c.entries[key]; ok {
		old := elem.Value.(*cacheEntry)
		c.memUsed -= old.size
		old.value = value
		old.size = size
		old.expires = time.Now().Add(c.ttl)
		c.memUsed += size
		c.order.MoveToFront(elem)
		return
	}

	// Evict if needed.
	for c.order.Len() >= c.maxEntries || (c.maxMem > 0 && c.memUsed+size > c.maxMem) {
		if c.order.Len() == 0 {
			break
		}
		c.removeLRU()
	}

	entry := &cacheEntry{
		key:     key,
		value:   value,
		size:    size,
		expires: time.Now().Add(c.ttl),
	}
	elem := c.order.PushFront(entry)
	c.entries[key] = elem
	c.memUsed += size
}

// InvalidateRepo removes all entries for a given repo.
func (c *Cache) InvalidateRepo(repo string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Build list of keys to remove (can't modify map while iterating).
	var toRemove []*list.Element
	for key, elem := range c.entries {
		// Keys are "repo:hash" so check prefix.
		if len(key) > len(repo) && key[:len(repo)+1] == repo+":" {
			toRemove = append(toRemove, elem)
		}
	}
	for _, elem := range toRemove {
		c.removeElement(elem)
	}
}

// Clear removes all entries.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*list.Element)
	c.order.Init()
	c.memUsed = 0
}

// Len returns the number of entries.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

func (c *Cache) removeLRU() {
	elem := c.order.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}

func (c *Cache) removeElement(elem *list.Element) {
	entry := elem.Value.(*cacheEntry)
	delete(c.entries, entry.key)
	c.order.Remove(elem)
	c.memUsed -= entry.size
}
