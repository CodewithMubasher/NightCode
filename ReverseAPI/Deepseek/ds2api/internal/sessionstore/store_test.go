package sessionstore

import (
	"sync"
	"testing"
)

func TestSessionStore_GetSetDelete(t *testing.T) {
	s := New()

	// Get non-existent
	if _, ok := s.Get("chat-1"); ok {
		t.Error("expected no session for non-existent chat")
	}

	// Set and get
	s.Set("chat-1", "session-abc")
	if sid, ok := s.Get("chat-1"); !ok || sid != "session-abc" {
		t.Errorf("expected session-abc, got %q (ok=%v)", sid, ok)
	}

	// Has
	if !s.Has("chat-1") {
		t.Error("expected Has to return true")
	}
	if s.Has("chat-2") {
		t.Error("expected Has to return false")
	}

	// Delete
	s.Delete("chat-1")
	if _, ok := s.Get("chat-1"); ok {
		t.Error("expected session to be deleted")
	}
}

func TestSessionStore_Isolation(t *testing.T) {
	s := New()

	s.Set("chat-a", "session-A")
	s.Set("chat-b", "session-B")

	// Verify isolation
	sidA, _ := s.Get("chat-a")
	sidB, _ := s.Get("chat-b")

	if sidA != "session-A" {
		t.Errorf("chat-a should have session-A, got %q", sidA)
	}
	if sidB != "session-B" {
		t.Errorf("chat-b should have session-B, got %q", sidB)
	}

	// Overwrite chat-a
	s.Set("chat-a", "session-A2")
	sidA, _ = s.Get("chat-a")
	if sidA != "session-A2" {
		t.Errorf("chat-a should have session-A2 after overwrite, got %q", sidA)
	}

	// chat-b unchanged
	sidB, _ = s.Get("chat-b")
	if sidB != "session-B" {
		t.Errorf("chat-b should still have session-B, got %q", sidB)
	}
}

func TestSessionStore_Concurrent(t *testing.T) {
	s := New()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			chatID := "chat-" + string(rune('A'+i%26))
			sessionID := "session-" + string(rune('0'+i%10))
			s.Set(chatID, sessionID)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			chatID := "chat-" + string(rune('A'+i%26))
			s.Get(chatID)
			s.Has(chatID)
		}(i)
	}

	wg.Wait()

	// Verify all chats have sessions
	for i := 0; i < 26; i++ {
		chatID := "chat-" + string(rune('A'+i))
		if !s.Has(chatID) {
			t.Errorf("expected session for %s", chatID)
		}
	}
}
