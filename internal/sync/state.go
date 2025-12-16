// Package sync provides file synchronization functionality.
package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// FileState represents the state of a synced file.
type FileState struct {
	Checksum   string    `json:"checksum"`
	Size       int64     `json:"size"`
	ModTime    time.Time `json:"mod_time"`
	LastSynced time.Time `json:"last_synced"`
	Direction  string    `json:"direction"`
	Rule       string    `json:"rule"`
}

// State tracks the state of all synced files.
type State struct {
	Files    map[string]FileState `json:"files"`
	LastPoll time.Time            `json:"last_poll"`
	mu       sync.RWMutex
	path     string
}

// NewState creates a new State that persists to the given path.
func NewState(path string) *State {
	return &State{
		Files: make(map[string]FileState),
		path:  path,
	}
}

// Load reads the state from disk.
func (s *State) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			// No state file yet, start fresh
			return nil
		}
		return fmt.Errorf("failed to read state file: %w", err)
	}

	if err := json.Unmarshal(data, s); err != nil {
		return fmt.Errorf("failed to parse state file: %w", err)
	}

	if s.Files == nil {
		s.Files = make(map[string]FileState)
	}

	return nil
}

// Save writes the state to disk.
func (s *State) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// Get returns the state of a file.
func (s *State) Get(path string) (FileState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.Files[path]
	return state, ok
}

// Set updates the state of a file.
func (s *State) Set(path string, state FileState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Files[path] = state
}

// Delete removes a file from the state.
func (s *State) Delete(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.Files, path)
}

// HasChanged checks if a file has changed since it was last synced.
func (s *State) HasChanged(path string, size int64, modTime time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.Files[path]
	if !ok {
		// Never synced before
		return true
	}

	// Check size and modification time
	return state.Size != size || !state.ModTime.Equal(modTime)
}

// HasChangedWithChecksum checks if a file has changed using checksum comparison.
func (s *State) HasChangedWithChecksum(path, checksum string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.Files[path]
	if !ok {
		return true
	}

	return state.Checksum != checksum
}

// UpdateLastPoll updates the last poll timestamp.
func (s *State) UpdateLastPoll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.LastPoll = time.Now()
}

// GetLastPoll returns the last poll timestamp.
func (s *State) GetLastPoll() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.LastPoll
}

// RecordTransfer records a successful file transfer.
func (s *State) RecordTransfer(path string, size int64, modTime time.Time, checksum, direction, rule string) {
	s.Set(path, FileState{
		Checksum:   checksum,
		Size:       size,
		ModTime:    modTime,
		LastSynced: time.Now(),
		Direction:  direction,
		Rule:       rule,
	})
}

// CalculateChecksum calculates the SHA256 checksum of a file.
func CalculateChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// CalculateChecksumFromReader calculates the SHA256 checksum from a reader.
func CalculateChecksumFromReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// Clear removes all entries from the state.
func (s *State) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Files = make(map[string]FileState)
	s.LastPoll = time.Time{}
}

// Count returns the number of tracked files.
func (s *State) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.Files)
}

// All returns a copy of all file states.
func (s *State) All() map[string]FileState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copy := make(map[string]FileState, len(s.Files))
	for k, v := range s.Files {
		copy[k] = v
	}
	return copy
}
