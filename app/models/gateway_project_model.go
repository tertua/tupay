package models

import "time"

import "github.com/google/uuid"

// GatewayProject is a downstream service allowed to create payment intents through Tupay. Identity is derived server-side from the API key, never from client-supplied fields.
type GatewayProject struct {
	Slug string `gorm:"primaryKey;size:64" db:"slug" json:"slug" validate:"required,lte=64"`
	GatewayProjectOwner
	OrgID          uuid.UUID `gorm:"type:uuid;index" db:"org_id" json:"org_id"`
	Name           string    `gorm:"size:255" db:"name" json:"name" validate:"required,lte=255"`
	APIKeyHash     string    `gorm:"size:128;uniqueIndex" db:"api_key_hash" json:"-"`
	WebhookURL     string    `gorm:"size:1024" db:"webhook_url" json:"webhook_url" validate:"required,lte=1024"`
	WebhookSecret  string    `gorm:"size:128" db:"webhook_secret" json:"-"`
	DefaultGateway string    `gorm:"size:32" db:"default_gateway" json:"default_gateway"`
	IsActive       bool      `gorm:"default:true" db:"is_active" json:"is_active"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}

// CreateProjectInput is the admin payload for registering a project.
// Secrets are generated server-side and returned once.
type CreateProjectInput struct {
	Slug           string `json:"slug" validate:"required,lte=64"`
	Name           string `json:"name" validate:"required,lte=255"`
	WebhookURL     string `json:"webhook_url" validate:"required,lte=1024"`
	DefaultGateway string `json:"default_gateway" validate:"omitempty,lte=32"`
}

// UpdateProjectInput edits project metadata (not secrets).
type UpdateProjectInput struct {
	Name           string `json:"name" validate:"omitempty,lte=255"`
	WebhookURL     string `json:"webhook_url" validate:"omitempty,lte=1024"`
	DefaultGateway string `json:"default_gateway" validate:"omitempty,lte=32"`
	IsActive       *bool  `json:"is_active"`
}
