// Copyright 2026 — see LICENSE file for terms.
package gitprovider

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherInvalidatesOnChange(t *testing.T) {
	dir, _ := initTestRepo(t)

	cache := NewCache(CacheConfig{MaxEntries: 100, TTLSeconds: 60})

	// Pre-populate cache with some entries for this repo.
	cache.Put(dir, "obj1", "data1", 10)
	cache.Put(dir, "obj2", "data2", 10)

	if cache.Len() != 2 {
		t.Fatalf("expected 2 cache entries, got %d", cache.Len())
	}

	w, err := NewWatcher(cache)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if err := w.WatchRepo(dir); err != nil {
		t.Fatal(err)
	}
	w.Start()

	// Simulate a git operation by touching .git/refs/heads/dev.
	refsDir := filepath.Join(dir, ".git", "refs", "heads")
	os.MkdirAll(refsDir, 0o755)                                            //nolint:errcheck
	os.WriteFile(filepath.Join(refsDir, "dev"), []byte("abc123\n"), 0o644) //nolint:errcheck

	// Wait for debounce + some margin.
	time.Sleep(300 * time.Millisecond)

	// Cache should be invalidated.
	if cache.Len() != 0 {
		t.Errorf("expected cache to be invalidated (len=%d)", cache.Len())
	}
}

func TestWatcherDoesNotInvalidateOtherRepo(t *testing.T) {
	dir, _ := initTestRepo(t)

	cache := NewCache(CacheConfig{MaxEntries: 100, TTLSeconds: 60})
	cache.Put("/other/repo", "obj1", "data1", 10)

	w, err := NewWatcher(cache)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if err := w.WatchRepo(dir); err != nil {
		t.Fatal(err)
	}
	w.Start()

	// Trigger change in watched repo.
	refsDir := filepath.Join(dir, ".git", "refs", "heads")
	os.WriteFile(filepath.Join(refsDir, "trigger"), []byte("xyz\n"), 0o644) //nolint:errcheck

	time.Sleep(300 * time.Millisecond)

	// Other repo's cache should be untouched.
	if v := cache.Get("/other/repo", "obj1"); v != "data1" {
		t.Error("expected other repo's cache to be untouched")
	}
}
