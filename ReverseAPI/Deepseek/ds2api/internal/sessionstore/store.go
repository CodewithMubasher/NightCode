package sessionstore

import (
	"sync"
)

// Store maps NightCode chat IDs to DeepSeek session IDs.
// This allows one NightCode chat to map to exactly one DeepSeek conversation.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]string // chatID → deepseekSessionID
}

// New creates a new session store.
func New() *Store {
	return &Store{
		sessions: make(map[string]string),
	}
}

// Get returns the DeepSeek session ID for a NightCode chat ID.
// Returns ("", false) if no session exists.
func (s *Store) Get(chatID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sid, ok := s.sessions[chatID]
	return sid, ok
}

// Set stores a mapping from NightCode chat ID to DeepSeek session ID.
func (s *Store) Set(chatID, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[chatID] = sessionID
}

// Delete removes a mapping.
func (s *Store) Delete(chatID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, chatID)
}

// Has returns true if a mapping exists.
func (s *Store) Has(chatID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.sessions[chatID]
	return ok
}
