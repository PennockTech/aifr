// Copyright 2026 — see LICENSE file for terms.
package gitprovider

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const debounceWindow = 100 * time.Millisecond

// Watcher monitors git repository metadata files for changes and invalidates
// the cache when refs may have changed.
type Watcher struct {
	watcher *fsnotify.Watcher
	cache   *Cache
	repos   map[string]string // repo path → name
	mu      sync.Mutex
	done    chan struct{}
}

// NewWatcher creates a file watcher that invalidates the cache on git ref changes.
func NewWatcher(cache *Cache) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		watcher: fw,
		cache:   cache,
		repos:   make(map[string]string),
		done:    make(chan struct{}),
	}, nil
}

// WatchRepo starts watching a git repository for ref changes.
func (w *Watcher) WatchRepo(repoPath string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	gitDir := filepath.Join(repoPath, ".git")
	if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
		return nil // not a git repo or bare repo
	}

	w.repos[repoPath] = repoPath

	// Watch key files.
	paths := []string{
		gitDir,
		filepath.Join(gitDir, "refs"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			if err := w.watcher.Add(p); err != nil {
				slog.Debug("watcher: cannot watch", "path", p, "error", err)
			}
		}
	}

	// Also watch refs subdirs.
	refsDir := filepath.Join(gitDir, "refs")
	entries, _ := os.ReadDir(refsDir)
	for _, e := range entries {
		if e.IsDir() {
			subPath := filepath.Join(refsDir, e.Name())
			w.watcher.Add(subPath) //nolint:errcheck
		}
	}

	return nil
}

// Start begins processing filesystem events in a goroutine.
func (w *Watcher) Start() {
	go w.eventLoop()
}

// Close stops the watcher.
func (w *Watcher) Close() error {
	close(w.done)
	return w.watcher.Close()
}

func (w *Watcher) eventLoop() {
	var (
		timer   *time.Timer
		pending = make(map[string]bool) // repo paths with pending invalidation
		mu      sync.Mutex
	)

	for {
		select {
		case <-w.done:
			if timer != nil {
				timer.Stop()
			}
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Determine which repo this event belongs to.
			repoPath := w.findRepo(event.Name)
			if repoPath == "" {
				continue
			}

			mu.Lock()
			pending[repoPath] = true
			mu.Unlock()

			// Debounce: reset the timer.
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(debounceWindow, func() {
				mu.Lock()
				toInvalidate := make([]string, 0, len(pending))
				for rp := range pending {
					toInvalidate = append(toInvalidate, rp)
				}
				pending = make(map[string]bool)
				mu.Unlock()

				for _, rp := range toInvalidate {
					slog.Debug("watcher: invalidating cache", "repo", rp)
					w.cache.InvalidateRepo(rp)
				}
			})

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("watcher error", "error", err)
		}
	}
}

// findRepo determines which repo a file event belongs to.
func (w *Watcher) findRepo(path string) string {
	w.mu.Lock()
	defer w.mu.Unlock()

	for repoPath := range w.repos {
		gitDir := filepath.Join(repoPath, ".git")
		if len(path) >= len(gitDir) && path[:len(gitDir)] == gitDir {
			return repoPath
		}
	}
	return ""
}
