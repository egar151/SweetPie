// Package watcher provides real-time file system monitoring.
package watcher

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"sftp-sync/internal/rules"
	"sftp-sync/pkg/logger"
)

// Event represents a file change event.
type Event struct {
	Path      string
	Name      string
	Operation string
	Time      time.Time
}

// Handler is called when a file change is detected.
type Handler func(event Event)

// Watcher monitors directories for file changes.
type Watcher struct {
	watcher  *fsnotify.Watcher
	matcher  *rules.Matcher
	handler  Handler
	logger   *logger.Logger
	debounce map[string]time.Time
	mu       sync.Mutex
	debounceDuration time.Duration
}

// NewWatcher creates a new file system watcher.
func NewWatcher(matcher *rules.Matcher, handler Handler, log *logger.Logger) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		watcher:          fsWatcher,
		matcher:          matcher,
		handler:          handler,
		logger:           log.WithComponent("watcher"),
		debounce:         make(map[string]time.Time),
		debounceDuration: 500 * time.Millisecond, // Debounce rapid changes
	}, nil
}

// Watch adds a directory to be watched.
func (w *Watcher) Watch(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	if err := w.watcher.Add(absPath); err != nil {
		return err
	}

	w.logger.Info().Str("path", absPath).Msg("Watching directory")
	return nil
}

// Unwatch removes a directory from being watched.
func (w *Watcher) Unwatch(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	if err := w.watcher.Remove(absPath); err != nil {
		return err
	}

	w.logger.Info().Str("path", absPath).Msg("Stopped watching directory")
	return nil
}

// Start begins watching for file changes.
func (w *Watcher) Start(ctx context.Context) {
	w.logger.Info().Msg("File watcher started")

	go func() {
		for {
			select {
			case <-ctx.Done():
				w.logger.Info().Msg("File watcher stopping")
				return

			case event, ok := <-w.watcher.Events:
				if !ok {
					return
				}
				w.handleEvent(event)

			case err, ok := <-w.watcher.Errors:
				if !ok {
					return
				}
				w.logger.Error().Err(err).Msg("Watcher error")
			}
		}
	}()
}

// handleEvent processes a file system event.
func (w *Watcher) handleEvent(fsEvent fsnotify.Event) {
	// Only handle create and write events
	if !fsEvent.Has(fsnotify.Create) && !fsEvent.Has(fsnotify.Write) {
		return
	}

	filename := filepath.Base(fsEvent.Name)

	// Check if file matches any rule
	if _, ok := w.matcher.Match(filename); !ok {
		return
	}

	// Debounce rapid events for the same file
	if !w.shouldProcess(fsEvent.Name) {
		return
	}

	var operation string
	switch {
	case fsEvent.Has(fsnotify.Create):
		operation = "create"
	case fsEvent.Has(fsnotify.Write):
		operation = "write"
	default:
		operation = "change"
	}

	w.logger.Debug().
		Str("file", filename).
		Str("operation", operation).
		Msg("File change detected")

	event := Event{
		Path:      fsEvent.Name,
		Name:      filename,
		Operation: operation,
		Time:      time.Now(),
	}

	// Call handler
	if w.handler != nil {
		w.handler(event)
	}
}

// shouldProcess checks if an event should be processed (debouncing).
func (w *Watcher) shouldProcess(path string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	lastEvent, exists := w.debounce[path]
	now := time.Now()

	if exists && now.Sub(lastEvent) < w.debounceDuration {
		return false
	}

	w.debounce[path] = now

	// Clean up old entries
	for p, t := range w.debounce {
		if now.Sub(t) > time.Minute {
			delete(w.debounce, p)
		}
	}

	return true
}

// Close stops the watcher and releases resources.
func (w *Watcher) Close() error {
	w.logger.Info().Msg("Closing file watcher")
	return w.watcher.Close()
}

// WatchedPaths returns a list of currently watched paths.
func (w *Watcher) WatchedPaths() []string {
	return w.watcher.WatchList()
}

// SetDebounceDuration sets the debounce duration for rapid events.
func (w *Watcher) SetDebounceDuration(d time.Duration) {
	w.debounceDuration = d
}
