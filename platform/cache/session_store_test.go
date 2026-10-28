package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMemoryIncrCountsPerKey pins the counter contract behind login
// throttling: increments accumulate per key and each key starts at 1.
func TestMemoryIncrCountsPerKey(t *testing.T) {
	store := newMemoryStore()
	ctx := context.Background()

	n, err := store.Incr(ctx, "auth:fail:x", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	n, err = store.Incr(ctx, "auth:fail:x", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)

	n, err = store.Incr(ctx, "auth:fail:y", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	_, err = store.Incr(ctx, "auth:fail:x", 0)
	assert.ErrorIs(t, err, errInvalidTTL)
}

// TestMemoryIncrWindowResets proves a quiet window forgets the ladder instead
// of leaving a permanent counter that locks the account forever.
func TestMemoryIncrWindowResets(t *testing.T) {
	store := newMemoryStore()
	ctx := context.Background()

	_, err := store.Incr(ctx, "auth:fail:x", 20*time.Millisecond)
	require.NoError(t, err)
	time.Sleep(40 * time.Millisecond)

	n, err := store.Incr(ctx, "auth:fail:x", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}
