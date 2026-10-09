package queries

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"gorm.io/gorm"
)

// UserQueries struct for queries from User model.
type UserQueries struct {
	*gorm.DB
}

// findOne loads the first row from a pre-chained query, mapping a GORM
// not-found error through notFound (sql.ErrNoRows) so callers keep the
// sentinel they already handle.
func findOne[T any](tx *gorm.DB, dest *T) error {
	if err := tx.First(dest).Error; err != nil {
		return notFound(err)
	}
	return nil
}

// GetUserByID query for getting one User by given ID.
func (q *UserQueries) GetUserByID(id uuid.UUID) (models.User, error) {
	user := models.User{}
	return user, findOne(q.Where("id = ?", id), &user)
}

// GetUserByEmail query for getting one User by given Email. The input is
// normalized the same way registrations are (lowercase + trim) so a lookup can
// never miss an account that was stored normalized, and the oldest row wins for
// determinism on any legacy duplicate.
func (q *UserQueries) GetUserByEmail(email string) (models.User, error) {
	user := models.User{}
	return user, findOne(q.Where("email = ?", strings.ToLower(strings.TrimSpace(email))).Order("created_at ASC"), &user)
}

// CreateUser query for creating a new user by given email and password hash.
func (q *UserQueries) CreateUser(u *models.User) error {
	return q.Create(u).Error
}

// CountUsers returns the number of registered users.
func (q *UserQueries) CountUsers() (int64, error) {
	var count int64
	if err := q.Model(&models.User{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// ListUsers returns one page of users without loading any related data.
func (q *UserQueries) ListUsers(limit, offset int) ([]models.User, error) {
	users := make([]models.User, 0)
	if err := q.Order("created_at ASC").Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// UpdateUserRole changes a user's role.
func (q *UserQueries) UpdateUserRole(id uuid.UUID, role string) error {
	return q.Model(&models.User{}).Where("id = ?", id).Updates(map[string]any{
		"updated_at": time.Now(),
		"user_role":  role,
	}).Error
}

// UpdateUserProfile query for updating user display name.
func (q *UserQueries) UpdateUserProfile(id uuid.UUID, name string) error {
	return q.Model(&models.User{}).Where("id = ?", id).Updates(map[string]any{
		"updated_at": time.Now(),
		"name":       name,
	}).Error
}

// UpdateUserPassword query for updating user password hash.
func (q *UserQueries) UpdateUserPassword(id uuid.UUID, passwordHash string) error {
	return q.Model(&models.User{}).Where("id = ?", id).Updates(map[string]any{
		"updated_at":    time.Now(),
		"password_hash": passwordHash,
	}).Error
}

// CreatePasswordReset query for storing a password reset token.
func (q *UserQueries) CreatePasswordReset(userID uuid.UUID, token string, expiresAt time.Time) error {
	reset := &models.PasswordReset{
		Token:     token,
		UserID:    userID,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}
	return q.Create(reset).Error
}

// GetPasswordReset query for getting a password reset token.
func (q *UserQueries) GetPasswordReset(token string) (models.PasswordReset, error) {
	reset := models.PasswordReset{}
	return reset, findOne(q.Where("token = ?", token), &reset)
}

// DeletePasswordResetsByUser query for deleting all reset tokens of a user.
func (q *UserQueries) DeletePasswordResetsByUser(userID uuid.UUID) error {
	return q.Where("user_id = ?", userID).Delete(&models.PasswordReset{}).Error
}

// DeleteUserAccount removes a user and every row keyed to them (memberships,
// external identities, email verifications, password resets) in one
// transaction. Callers gate on CountInvoicesByUser first: invoice records
// stay on the books even after their creator is gone, so a user with any
// invoice record must never reach this delete.
func (q *UserQueries) DeleteUserAccount(userID uuid.UUID) error {
	return q.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&models.Membership{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserIdentity{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.EmailVerification{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.PasswordReset{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", userID).Delete(&models.User{}).Error
	})
}
