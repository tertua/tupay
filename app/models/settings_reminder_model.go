package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// SettingsReminder holds the per-org automatic payment reminder schedule. It is
// embedded in Settings so the base settings model stays small.
//
// ReminderBeforeDays / ReminderAfterDays count days around the invoice due date;
// 0 disables that leg entirely. ReminderEnabled is the org-wide switch.
type SettingsReminder struct {
	ReminderEnabled    bool `gorm:"default:true" db:"reminder_enabled" json:"reminder_enabled"`
	ReminderBeforeDays int  `gorm:"default:7" db:"reminder_before_days" json:"reminder_before_days" validate:"gte=0,lte=365"`
	ReminderAfterDays  int  `gorm:"default:3" db:"reminder_after_days" json:"reminder_after_days" validate:"gte=0,lte=365"`
}

// DefaultReminderSettings returns the first-run reminder schedule: enabled with
// one reminder 7 days before and one 3 days after the due date.
func DefaultReminderSettings() SettingsReminder {
	return SettingsReminder{ReminderEnabled: true, ReminderBeforeDays: 7, ReminderAfterDays: 3}
}

// DefaultSettings returns first-run defaults (tax rate, USD→IDR, invoice prefix,
// reminder schedule) keyed by userID; orgID rides as userID until register
// provisions the personal org (D7). It lives beside the reminder defaults to
// keep settings_model.go within its size baseline.
func DefaultSettings(userID uuid.UUID) *Settings {
	s := &Settings{OrgID: userID, UserID: userID, UpdatedAt: time.Now(), Currency: "IDR", TaxRate: 11, InvoicePrefix: "INV-"}
	s.UsdToIdr = decimal.NewFromInt(18000)
	s.SettingsReminder = DefaultReminderSettings()
	return s
}

