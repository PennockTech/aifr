// Copyright 2026 — see LICENSE file for terms.
package gitprovider

import (
	"testing"
)

func TestCacheBasic(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 10, MaxMemoryMB: 1, TTLSeconds: 60})

	c.Put("/repo", "abc123", "data1", 100)
	c.Put("/repo", "def456", "data2", 200)

	if c.Len() != 2 {
		t.Errorf("len = %d, want 2", c.Len())
	}

	v := c.Get("/repo", "abc123")
	if v != "data1" {
		t.Errorf("get = %v, want data1", v)
	}
}

func TestCacheMiss(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 10, TTLSeconds: 60})

	v := c.Get("/repo", "nonexistent")
	if v != nil {
		t.Errorf("expected nil for cache miss, got %v", v)
	}
}

func TestCacheEviction(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 3, MaxMemoryMB: 1, TTLSeconds: 60})

	c.Put("/repo", "a", "data-a", 10)
	c.Put("/repo", "b", "data-b", 10)
	c.Put("/repo", "c", "data-c", 10)

	// Adding a 4th should evict the LRU (a).
	c.Put("/repo", "d", "data-d", 10)

	if c.Len() != 3 {
		t.Errorf("len = %d, want 3", c.Len())
	}

	if v := c.Get("/repo", "a"); v != nil {
		t.Error("expected 'a' to be evicted")
	}
	if v := c.Get("/repo", "d"); v != "data-d" {
		t.Errorf("expected 'd' to be present, got %v", v)
	}
}

func TestCacheLRUOrder(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 3, MaxMemoryMB: 1, TTLSeconds: 60})

	c.Put("/repo", "a", "data-a", 10)
	c.Put("/repo", "b", "data-b", 10)
	c.Put("/repo", "c", "data-c", 10)

	// Access 'a' to make it most recently used.
	c.Get("/repo", "a")

	// Add 'd' — should evict 'b' (least recently used).
	c.Put("/repo", "d", "data-d", 10)

	if v := c.Get("/repo", "b"); v != nil {
		t.Error("expected 'b' to be evicted")
	}
	if v := c.Get("/repo", "a"); v != "data-a" {
		t.Errorf("expected 'a' to still be present, got %v", v)
	}
}

func TestCacheInvalidateRepo(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 100, TTLSeconds: 60})

	c.Put("/repo1", "a", "r1-a", 10)
	c.Put("/repo1", "b", "r1-b", 10)
	c.Put("/repo2", "a", "r2-a", 10)

	c.InvalidateRepo("/repo1")

	if c.Len() != 1 {
		t.Errorf("len = %d, want 1", c.Len())
	}
	if v := c.Get("/repo1", "a"); v != nil {
		t.Error("expected repo1 entries to be removed")
	}
	if v := c.Get("/repo2", "a"); v != "r2-a" {
		t.Errorf("expected repo2 entries to remain, got %v", v)
	}
}

func TestCacheClear(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 100, TTLSeconds: 60})

	c.Put("/repo", "a", "data", 10)
	c.Put("/repo", "b", "data", 10)

	c.Clear()

	if c.Len() != 0 {
		t.Errorf("len = %d, want 0 after clear", c.Len())
	}
}

func TestCacheMemoryEviction(t *testing.T) {
	// Cache with 1 MB limit.
	c := NewCache(CacheConfig{MaxEntries: 1000, MaxMemoryMB: 1, TTLSeconds: 60})

	// Add entries totaling more than 1 MB.
	bigData := make([]byte, 512*1024) // 512 KiB
	c.Put("/repo", "a", string(bigData), int64(len(bigData)))
	c.Put("/repo", "b", string(bigData), int64(len(bigData)))

	// Should still be at 2 entries (1 MB total = within limit).
	if c.Len() != 2 {
		t.Errorf("len = %d, want 2", c.Len())
	}

	// Adding a third should evict to stay under memory limit.
	c.Put("/repo", "c", string(bigData), int64(len(bigData)))

	if c.Len() > 2 {
		t.Errorf("len = %d, want <= 2 (memory limit)", c.Len())
	}
}

func TestCacheUpdate(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 10, TTLSeconds: 60})

	c.Put("/repo", "a", "v1", 10)
	c.Put("/repo", "a", "v2", 20)

	if c.Len() != 1 {
		t.Errorf("len = %d, want 1 (update should not add entry)", c.Len())
	}
	if v := c.Get("/repo", "a"); v != "v2" {
		t.Errorf("expected updated value 'v2', got %v", v)
	}
}
