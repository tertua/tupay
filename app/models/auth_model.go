package models

import (
	"time"

	"github.com/google/uuid"
)

// Register struct to describe register a new user.
type Register struct {
	Name        string `json:"name" validate:"required,lte=255"`
	Email       string `json:"email" validate:"required,email,lte=255"`
	Password    string `json:"password" validate:"required,min=8,lte=255"`
	InviteToken string `json:"invite_token,omitempty" validate:"omitempty"`
}

// UpdateProfile struct to describe update profile payload.
type UpdateProfile struct {
	Name string `json:"name" validate:"required,lte=255"`
}

// ChangePassword struct to describe change password payload.
type ChangePassword struct {
	CurrentPassword string `json:"currentPassword" validate:"required,lte=255"`
	NewPassword     string `json:"newPassword" validate:"required,min=8,lte=255"`
}

// ForgotPassword struct to describe forgot password payload.
type ForgotPassword struct {
	Email string `json:"email" validate:"required,email,lte=255"`
}

// ResetPassword struct to describe reset password payload.
type ResetPassword struct {
	Token       string `json:"token" validate:"required,lte=255"`
	NewPassword string `json:"new_password" validate:"required,min=8,lte=255"`
}

// PasswordReset struct to describe a stored password reset token.
type PasswordReset struct {
	Token     string    `gorm:"primaryKey" db:"token" json:"token"`
	UserID    uuid.UUID `gorm:"type:uuid" db:"user_id" json:"user_id"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	ExpiresAt time.Time `db:"expires_at" json:"expires_at"`
}
