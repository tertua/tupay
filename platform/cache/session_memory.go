package cache

import (
	"context"
	"strconv"
	"sync"
	"time"
)

// memoryEntry is one in-memory session or counter with expiry.
type memoryEntry struct {
	value   string
	expires time.Time
}

// memoryStore is a SessionStore backed by a guarded map.
type memoryStore struct {
	mu    sync.Mutex
	items map[string]memoryEntry
}

func newMemoryStore() *memoryStore {
	store := &memoryStore{items: make(map[string]memoryEntry)}
	go store.janitor()
	return store
}

func (s *memoryStore) Get(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.items[key]
	if !ok || time.Now().After(entry.expires) {
		delete(s.items, key)
		return "", ErrSessionNotFound
	}
	return entry.value, nil
}

func (s *memoryStore) Set(_ context.Context, key, value string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items[key] = memoryEntry{value: value, expires: time.Now().Add(ttl)}
	return nil
}

func (s *memoryStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.items, key)
	return nil
}

// Incr keeps the map's single lock across read+write, so concurrent increments
// cannot lose a count the way an unlocked Get+Set would.
func (s *memoryStore) Incr(_ context.Context, key string, ttl time.Duration) (int64, error) {
	if ttl <= 0 {
		return 0, errInvalidTTL
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	entry, ok := s.items[key]
	if !ok || now.After(entry.expires) {
		s.items[key] = memoryEntry{value: "1", expires: now.Add(ttl)}
		return 1, nil
	}
	n, err := strconv.ParseInt(entry.value, 10, 64)
	if err != nil {
		n = 0
	}
	n++
	entry.value = strconv.FormatInt(n, 10)
	s.items[key] = entry
	return n, nil
}

// janitor removes expired sessions every minute.
func (s *memoryStore) janitor() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		s.mu.Lock()
		for key, entry := range s.items {
			if now.After(entry.expires) {
				delete(s.items, key)
			}
		}
		s.mu.Unlock()
	}
}
