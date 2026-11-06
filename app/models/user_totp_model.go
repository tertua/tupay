package models

import (
	"time"

	"github.com/google/uuid"
)

// TOTPPeriod is the RFC 6238 time step in seconds; it is also the unit of
// LastUsedStep, which records the step a code was spent on so a replay inside
// the same window is rejected.
const TOTPPeriod = 30

// UserTOTP is the second factor of one account: the sealed TOTP secret plus
// the last step consumed, so a code can never be presented twice.
type UserTOTP struct {
	UserID       uuid.UUID  `gorm:"type:uuid;primaryKey" db:"user_id" json:"user_id"`
	Secret       string     `gorm:"size:512" db:"secret" json:"-"`
	LastUsedStep int64      `db:"last_used_step" json:"last_used_step"`
	ActivatedAt  *time.Time `db:"activated_at" json:"activated_at"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
}

// TableName keeps the plural table convention used by the other models.
func (UserTOTP) TableName() string { return "user_totps" }

// TOTPEnrollResponse is what an enrollment returns: the shared secret and the
// otpauth:// URL the authenticator app scans. Neither is stored in plaintext.
type TOTPEnrollResponse struct {
	Secret     string `json:"secret"`
	OtpauthURL string `json:"otpauth_url"`
}

// TOTPActivateInput confirms enrollment with a first valid code.
type TOTPActivateInput struct {
	Code string `json:"code" validate:"required,len=6,numeric"`
}

// TOTPDisableInput turns the second factor off; both factors are required so a
// stolen session alone cannot weaken the account.
type TOTPDisableInput struct {
	Password string `json:"password" validate:"required,lte=255"`
	Code     string `json:"code" validate:"required,len=6,numeric"`
}

// TOTPStatus is the read model the settings screen renders.
type TOTPStatus struct {
	Enabled bool `json:"enabled"`
}
