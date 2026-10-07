package models

import (
	"time"

	"github.com/google/uuid"
)

// ClientLink exposes one client's invoice history through a public token.
// The row is keyed by the token hash (sha256 hex, never the raw token), so a
// database leak exposes no usable link. A NULL RevokedAt means live; a
// non-nil value is an audit tombstone that every lookup filters out.
type ClientLink struct {
	TokenHash string     `gorm:"primaryKey;size:64" db:"token_hash" json:"-"` // sha256 hex, never the raw token
	ClientID  uuid.UUID  `gorm:"type:uuid;uniqueIndex" db:"client_id" json:"client_id"`
	OrgID     uuid.UUID  `gorm:"type:uuid;index" db:"org_id" json:"org_id"`
	UserID    uuid.UUID  `gorm:"type:uuid;index" db:"user_id" json:"user_id"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	RevokedAt *time.Time `db:"revoked_at" json:"revoked_at"`
}
