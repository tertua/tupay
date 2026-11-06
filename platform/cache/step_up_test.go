package cache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStepUpKey(t *testing.T) {
	if got := StepUpKey("abc123"); got != "auth:stepup:abc123" {
		t.Fatalf("unexpected key %q", got)
	}
}

func TestStepUpValueRoundTrip(t *testing.T) {
	userID := uuid.New()
	expires := time.Unix(1700000000, 0)
	value := EncodeStepUpValue(userID, "sid-1", expires)
	gotUser, gotSID, ok := DecodeStepUpValue(value, time.Unix(1699999999, 0))
	if !ok || gotUser != userID || gotSID != "sid-1" {
		t.Fatalf("roundtrip mismatch: %v %q %v", gotUser, gotSID, ok)
	}
}

func TestStepUpValueExpired(t *testing.T) {
	value := EncodeStepUpValue(uuid.New(), "sid-1", time.Unix(1700000000, 0))
	if _, _, ok := DecodeStepUpValue(value, time.Unix(1700000000, 0)); ok {
		t.Fatal("a token must not be valid at its own expiry instant")
	}
}

func TestStepUpValueMalformed(t *testing.T) {
	user := uuid.New().String()
	tests := []struct {
		name  string
		value string
	}{
		{"empty value", ""},
		{"two parts", user + "|sid-1"},
		{"four parts", user + "|sid-1|1700000000|extra"},
		{"bad number", user + "|sid-1|notanumber"},
		{"bad user", "not-a-uuid|sid-1|1700000000"},
		{"nil user", uuid.Nil.String() + "|sid-1|1700000000"},
		{"empty sid", user + "||1700000000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUser, gotSID, ok := DecodeStepUpValue(tt.value, time.Unix(1, 0))
			if ok || gotUser != uuid.Nil || gotSID != "" {
				t.Fatalf("expected ok=false for %q, got %v %q %v", tt.value, gotUser, gotSID, ok)
			}
		})
	}
}

// TestMemoryTakeAtomic proves Take is single-use: a second call on the same key
// returns ErrSessionNotFound.
func TestMemoryTakeAtomic(t *testing.T) {
	store := newMemoryStore()
	ctx := context.Background()
	key := "auth:stepup:test-token"
	want := EncodeStepUpValue(uuid.New(), "sid-1", time.Now().Add(time.Hour))

	require.NoError(t, store.Set(ctx, key, want, time.Hour))

	got, err := store.Take(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	_, err = store.Take(ctx, key)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

// TestMemoryTakeExpired proves an expired key is treated as absent.
// We insert directly into the map so Set does not overwrite the past expiry.
func TestMemoryTakeExpired(t *testing.T) {
	store := newMemoryStore()
	ctx := context.Background()
	key := "auth:stepup:expired-token"
	want := EncodeStepUpValue(uuid.New(), "sid-1", time.Now().Add(-time.Hour))

	// Insert directly so the expiry is already in the past.
	store.items[key] = memoryEntry{value: want, expires: time.Now().Add(-time.Hour)}

	_, err := store.Take(ctx, key)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

// TestMemoryTakeConcurrent races two goroutines on the same key; exactly one
// must succeed and the other must get ErrSessionNotFound.
func TestMemoryTakeConcurrent(t *testing.T) {
	store := newMemoryStore()
	ctx := context.Background()
	key := "auth:stepup:race-token"
	want := EncodeStepUpValue(uuid.New(), "sid-1", time.Now().Add(time.Hour))
	require.NoError(t, store.Set(ctx, key, want, time.Hour))

	results := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			_, err := store.Take(ctx, key)
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	var wins, loses int
	for err := range results {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, ErrSessionNotFound):
			loses++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}

	assert.Equal(t, 1, wins, "exactly one goroutine must win")
	assert.Equal(t, 1, loses, "exactly one goroutine must lose")
}
