package cache

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/tertua/tupay/pkg/configs"
)

// ErrSessionNotFound is returned when a session key does not exist.
var ErrSessionNotFound = errors.New("session not found")

// SessionStore is the shared TTL key/value store: refresh sessions plus the
// counters behind per-account sign-in throttling.
type SessionStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	// Incr atomically increments key and returns the new value, arming ttl on
	// the first hit only. A Get+Set pair would lose counts across instances,
	// which is exactly what a brute-force counter cannot afford.
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

var (
	storeOnce   sync.Once
	sharedStore SessionStore
	storeErr    error
)

// Sessions returns the shared store: Redis when REDIS_HOST is set, in-memory
// otherwise (single instance, suitable for dev without Redis).
func Sessions() (SessionStore, error) {
	storeOnce.Do(func() {
		if !configs.Get().Redis.Enabled() {
			sharedStore = newMemoryStore()
			return
		}
		client, err := RedisConnection()
		if err != nil {
			storeErr = err
			return
		}
		sharedStore = &redisStore{client: client}
	})
	return sharedStore, storeErr
}

// incrScript arms the key's TTL only when it is created, so a fixed counter
// window cannot be stretched by repeated failures.
var incrScript = redis.NewScript(`
local n = redis.call("INCR", KEYS[1])
if n == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return n
`)

// errInvalidTTL rejects a zero/negative window instead of writing a key that
// never expires (silently disabling the limiter it backs).
var errInvalidTTL = errors.New("cache: ttl must be positive")

// redisStore is a SessionStore backed by Redis.
type redisStore struct {
	client *redis.Client
}

func (s *redisStore) Get(ctx context.Context, key string) (string, error) {
	value, err := s.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrSessionNotFound
	}
	return value, err
}

func (s *redisStore) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return s.client.Set(ctx, key, value, ttl).Err()
}

func (s *redisStore) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

func (s *redisStore) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	if ttl <= 0 {
		return 0, errInvalidTTL
	}
	return incrScript.Run(ctx, s.client, []string{key}, ttl.Milliseconds()).Int64()
}
