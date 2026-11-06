package cache

import (
	"testing"
	"time"

	"github.com/google/uuid"
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
