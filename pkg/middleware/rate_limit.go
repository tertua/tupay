package middleware

import (
	"context"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/redis/go-redis/v9"
	"github.com/tertua/tupay/pkg/configs"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/cache"
	"github.com/tertua/tupay/platform/relay"
)

var rateLimitWindow = time.Minute

var redisStorageOnce sync.Once
var redisStorage fiber.Storage

// rateLimitStorage returns nil (fiber in-memory store) when REDIS_HOST is
// empty, otherwise a Redis-backed fiber.Storage so limits are shared across
// replicas. Nil is a valid Storage for limiter.New (uses process memory).
var rateLimitStorage = func() fiber.Storage {
	if !configs.Get().Redis.Enabled() {
		return nil
	}
	redisStorageOnce.Do(func() {
		client, err := cache.RedisConnection()
		if err != nil {
			return
		}
		redisStorage = newRedisRateLimitStorage(client)
	})
	return redisStorage
}

// limitReached keeps the shared {"error":{...}} envelope so the frontend
// apiClient interceptor reads the server message correctly.
func limitReached(c fiber.Ctx) error {
	c.Set("Retry-After", "60")
	return utils.Fail(c, fiber.StatusTooManyRequests, "rate limit exceeded, try again later", nil)
}

// newLimiter namespaces keys per class because all limiters share one storage; bare IP keys would merge every counter.
func newLimiter(maxRequests int, keyGen func(fiber.Ctx) string) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          maxRequests,
		Expiration:   rateLimitWindow,
		KeyGenerator: keyGen,
		LimitReached: limitReached,
		Storage:      rateLimitStorage(),
	})
}

// GeneralLimiter guards authenticated /api traffic.
func GeneralLimiter() fiber.Handler {
	return newLimiter(configs.Get().RateLimit.General, func(c fiber.Ctx) string { return "gen:" + c.IP() })
}

// AuthLimiter guards brute-forceable auth endpoints.
func AuthLimiter() fiber.Handler {
	return newLimiter(configs.Get().RateLimit.Auth, func(c fiber.Ctx) string { return "auth:" + c.IP() })
}

// PublicPayLimiter guards the public payment pages.
func PublicPayLimiter() fiber.Handler {
	return newLimiter(configs.Get().RateLimit.Public, func(c fiber.Ctx) string { return "pub:" + c.IP() })
}

// PublicClientLimiter guards the public client portal. A distinct key prefix
// keeps a busy pay page from starving the portal (they share one storage).
func PublicClientLimiter() fiber.Handler {
	return newLimiter(configs.Get().RateLimit.Public, func(c fiber.Ctx) string { return "pubclient:" + c.IP() })
}

// WebhookLimiter guards provider webhooks.
func WebhookLimiter() fiber.Handler {
	return newLimiter(configs.Get().RateLimit.Webhook, func(c fiber.Ctx) string { return "wh:" + c.IP() })
}

// GatewayLimiter guards the service relay (per API key, IP fallback).
// Keying by API key is fairer than IP when one downstream fans out.
func GatewayLimiter() fiber.Handler {
	return newLimiter(configs.Get().RateLimit.Gateway, func(c fiber.Ctx) string {
		if key := relay.ExtractKey(c.Get("Authorization"), c.Get("X-Api-Key")); key != "" {
			return "gw:" + relay.HashKey(key)
		}
		return "gw:" + c.IP()
	})
}

// redisRateLimitStorage adapts go-redis to fiber.Storage for the limiter.
// Keys are prefixed to avoid colliding with session keys.
type redisRateLimitStorage struct {
	client *redis.Client
	prefix string
}

func newRedisRateLimitStorage(client *redis.Client) *redisRateLimitStorage {
	return &redisRateLimitStorage{client: client, prefix: "ratelimit:"}
}

func (s *redisRateLimitStorage) key(k string) string { return s.prefix + k }

func (s *redisRateLimitStorage) Get(key string) ([]byte, error) {
	return s.GetWithContext(context.Background(), key)
}

func (s *redisRateLimitStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	val, err := s.client.Get(ctx, s.key(key)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return val, err
}

func (s *redisRateLimitStorage) Set(key string, val []byte, exp time.Duration) error {
	return s.SetWithContext(context.Background(), key, val, exp)
}

func (s *redisRateLimitStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	if key == "" || len(val) == 0 {
		return nil
	}
	return s.client.Set(ctx, s.key(key), val, exp).Err()
}

func (s *redisRateLimitStorage) Delete(key string) error {
	return s.DeleteWithContext(context.Background(), key)
}

func (s *redisRateLimitStorage) DeleteWithContext(ctx context.Context, key string) error {
	return s.client.Del(ctx, s.key(key)).Err()
}

// Reset removes this middleware's keys only (scan by prefix), never FLUSHDB.
func (s *redisRateLimitStorage) Reset() error {
	return s.ResetWithContext(context.Background())
}

func (s *redisRateLimitStorage) ResetWithContext(ctx context.Context) error {
	var cursor uint64
	for {
		keys, next, err := s.client.Scan(ctx, cursor, s.prefix+"*", 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := s.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

func (s *redisRateLimitStorage) Close() error {
	return s.client.Close()
}
