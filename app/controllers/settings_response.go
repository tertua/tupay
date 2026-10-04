package controllers

import (
	"time"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
)

// settingsPayload renders settings for the API. MidtransMethods is the
// DEPRECATED alias for ProviderMethods: both are assigned from
// s.ProviderMethods so the two keys always agree (mirrors how intentResponse
// keeps snap_token) until old relay/dashboard clients migrate.
func settingsPayload(s models.Settings) settingsResponse {
	return settingsResponse{
		OrgID:              s.OrgID,
		UserID:             s.UserID,
		UpdatedAt:          s.UpdatedAt,
		CompanyName:        s.CompanyName,
		Email:              s.Email,
		Phone:              s.Phone,
		Address:            s.Address,
		LogoURL:            s.LogoURL,
		Currency:           s.Currency,
		TaxRate:            s.TaxRate,
		UsdToIDR:           s.UsdToIdr,
		ProviderMethods:    s.ProviderMethods,
		MidtransMethods:    s.ProviderMethods,
		InvoicePrefix:      s.InvoicePrefix,
		Language:           s.Language,
		ReminderEnabled:    s.ReminderEnabled,
		ReminderBeforeDays: s.ReminderBeforeDays,
		ReminderAfterDays:  s.ReminderAfterDays,
	}
}

// settingsResponse is the settings payload. Field order mirrors the historical
// hand-built map; MidtransMethods is a deprecated alias kept in lockstep with
// ProviderMethods inside settingsPayload.
type settingsResponse struct {
	OrgID              uuid.UUID    `json:"org_id"`
	UserID             uuid.UUID    `json:"user_id"`
	UpdatedAt          time.Time    `json:"updated_at"`
	CompanyName        string       `json:"company_name"`
	Email              string       `json:"email"`
	Phone              string       `json:"phone"`
	Address            string       `json:"address"`
	LogoURL            string       `json:"logo_url"`
	Currency           string       `json:"currency"`
	TaxRate            float64      `json:"tax_rate"`
	UsdToIDR           models.Money `json:"usd_to_idr"`
	ProviderMethods    string       `json:"provider_methods"`
	MidtransMethods    string       `json:"midtrans_methods"`
	InvoicePrefix      string       `json:"invoice_prefix"`
	Language           string       `json:"language"`
	ReminderEnabled    bool         `json:"reminder_enabled"`
	ReminderBeforeDays int          `json:"reminder_before_days"`
	ReminderAfterDays  int          `json:"reminder_after_days"`
}
