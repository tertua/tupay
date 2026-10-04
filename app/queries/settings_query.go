package queries

import (
	"time"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"gorm.io/gorm"
)

// SettingsQueries struct for queries from Settings model.
type SettingsQueries struct {
	*gorm.DB
}

// GetSettings returns org settings, creating defaults on first use.
func (q *SettingsQueries) GetSettings(orgID uuid.UUID) (models.Settings, error) {
	settings := models.Settings{OrgID: orgID}
	// No actor param here, so user_id rides as orgID until register provisions the personal org (6.11).
	if err := q.Where("org_id = ?", orgID).
		Attrs(models.DefaultSettings(orgID)).
		FirstOrCreate(&settings).Error; err != nil {
		return settings, err
	}
	return settings, nil
}

// CreateSettings creates the default settings row for an org, leaving an existing row untouched (an invite join reuses the inviting org's settings).
func (q *SettingsQueries) CreateSettings(s *models.Settings) error {
	return q.Where("org_id = ?", s.OrgID).Attrs(*s).FirstOrCreate(s).Error
}

// CreateSettingsTx is CreateSettings inside a caller-owned transaction (OIDC provisioning shares one tx).
func CreateSettingsTx(tx *gorm.DB, s *models.Settings) error {
	return tx.Where("org_id = ?", s.OrgID).Attrs(*s).FirstOrCreate(s).Error
}

// UpdateSettings updates org settings.
func (q *SettingsQueries) UpdateSettings(s *models.Settings) error {
	if err := q.Model(&models.Settings{}).Where("org_id = ?", s.OrgID).
		Updates(map[string]any{
			"updated_at":           time.Now(),
			"company_name":         s.CompanyName,
			"email":                s.Email,
			"phone":                s.Phone,
			"address":              s.Address,
			"logo_url":             s.LogoURL,
			"currency":             s.Currency,
			"tax_rate":             s.TaxRate,
			"usd_to_idr":           s.UsdToIdr,
			"provider_methods":     s.ProviderMethods,
			"invoice_prefix":       s.InvoicePrefix,
			"language":             s.Language,
			"reminder_enabled":     s.ReminderEnabled,
			"reminder_before_days": s.ReminderBeforeDays,
			"reminder_after_days":  s.ReminderAfterDays,
		}).Error; err != nil {
		return err
	}

	return nil
}
