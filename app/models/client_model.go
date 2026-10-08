package models

import (
	"time"

	"github.com/google/uuid"
)

// Client describes a client.
type Client struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" db:"id" json:"id" validate:"required,uuid"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt *time.Time `db:"updated_at" json:"updated_at"`
	UserID    uuid.UUID  `gorm:"type:uuid" db:"user_id" json:"user_id" validate:"required,uuid"`
	OrgID     uuid.UUID  `gorm:"type:uuid;index" db:"org_id" json:"org_id"`
	ClientGatewayIdentity
	Name    string `db:"name" json:"name" validate:"required,lte=255"`
	Email   string `db:"email" json:"email" validate:"omitempty,email,lte=255"`
	Company string `db:"company" json:"company" validate:"lte=255"`
	Phone   string `db:"phone" json:"phone" validate:"lte=100"`
	Address string `db:"address" json:"address"`
	Notes   string `db:"notes" json:"notes"`
	Status  string `db:"status" json:"status" gorm:"size:16;default:active;index" validate:"omitempty,oneof=active archived"`
}
