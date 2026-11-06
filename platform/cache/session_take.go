package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// takeStore is the atomic take capability used by single-use values (step-up
// tokens): read and delete happen as one operation, so two concurrent requests
// presenting the same key cannot both succeed.
type takeStore interface {
	Take(ctx context.Context, key string) (string, error)
}

// Take reads and deletes a key atomically (Redis GETDEL).  If the key is absent
// or expired, ErrSessionNotFound is returned.
func (s *redisStore) Take(ctx context.Context, key string) (string, error) {
	value, err := s.client.GetDel(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrSessionNotFound
	}
	return value, err
}

// Take keeps the map's single lock across read+delete, so only one goroutine
// can claim a given key.
func (s *memoryStore) Take(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.items[key]
	if !ok || time.Now().After(entry.expires) {
		delete(s.items, key)
		return "", ErrSessionNotFound
	}
	delete(s.items, key)
	return entry.value, nil
}
