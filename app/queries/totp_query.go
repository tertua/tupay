package queries

import (
	"time"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"gorm.io/gorm"
)

// TOTPQueries struct for queries from the UserTOTP model.
type TOTPQueries struct {
	*gorm.DB
}

// GetTOTP returns the second-factor row of an account.
func (q *TOTPQueries) GetTOTP(userID uuid.UUID) (models.UserTOTP, error) {
	row := models.UserTOTP{}
	if err := q.Where("user_id = ?", userID).First(&row).Error; err != nil {
		return row, notFound(err)
	}
	return row, nil
}

// TOTPEnrolled reports whether an account has an activated second factor. A
// pending (unactivated) enrollment does not count, so login stays single-factor
// until the user proves the authenticator works.  It fails closed: a DB error
// is returned so the caller can refuse the request instead of bypassing the gate.
func (q *TOTPQueries) TOTPEnrolled(userID uuid.UUID) (bool, error) {
	var count int64
	err := q.Model(&models.UserTOTP{}).
		Where("user_id = ? AND activated_at IS NOT NULL", userID).
		Count(&count).Error
	return err == nil && count > 0, err
}

// UpsertTOTP stores a fresh (unactivated) secret, replacing any pending row so
// re-enrolling before activation cannot leave two live secrets.
func (q *TOTPQueries) UpsertTOTP(userID uuid.UUID, sealed string) error {
	now := time.Now()
	return DoRetry(func() error {
		return q.Transaction(func(tx *gorm.DB) error {
			return tx.Where("user_id = ?", userID).
				Assign(models.UserTOTP{Secret: sealed, LastUsedStep: 0, ActivatedAt: nil, UpdatedAt: now}).
				FirstOrCreate(&models.UserTOTP{UserID: userID, CreatedAt: now}).Error
		})
	})
}

// ActivateTOTP marks the pending secret as confirmed.
func (q *TOTPQueries) ActivateTOTP(userID uuid.UUID) error {
	return q.Model(&models.UserTOTP{}).
		Where("user_id = ?", userID).
		Updates(map[string]any{"activated_at": time.Now(), "updated_at": time.Now()}).Error
}

// DeleteTOTP removes the second factor of an account.
func (q *TOTPQueries) DeleteTOTP(userID uuid.UUID) error {
	return q.Where("user_id = ?", userID).Delete(&models.UserTOTP{}).Error
}

// SetTOTPLastStep records the step a code was spent on, so the same code
// cannot be replayed inside its validity window.
func (q *TOTPQueries) SetTOTPLastStep(userID uuid.UUID, step int64) error {
	return q.Model(&models.UserTOTP{}).
		Where("user_id = ?", userID).
		Updates(map[string]any{"last_used_step": step, "updated_at": time.Now()}).Error
}
