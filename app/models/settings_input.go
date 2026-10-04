package models

// SettingsInput describes the user-editable settings payload.
type SettingsInput struct {
	CompanyName string  `json:"company_name" validate:"lte=255"`
	Email       string  `json:"email" validate:"omitempty,email,lte=255"`
	Phone       string  `json:"phone" validate:"lte=100"`
	Address     string  `json:"address"`
	LogoURL     string  `json:"logo_url"`
	Currency    string  `json:"currency" validate:"required,lte=3"`
	TaxRate     float64 `json:"tax_rate" validate:"gte=0"`
	UsdToIdr    Money   `json:"usd_to_idr"`
	// ProviderMethods is the single legacy gateway method id for the default
	// provider. normalizeProviderMethods pins it to the provider's declared
	// default (qris for the default provider); other providers keep the raw
	// trimmed value.
	ProviderMethods string `json:"provider_methods" validate:"omitempty,lte=255"`
	// MidtransMethods is the DEPRECATED input alias for ProviderMethods, kept
	// until clients migrate; provider_methods wins when both are sent.
	MidtransMethods    string `json:"midtrans_methods" validate:"omitempty,lte=255"`
	InvoicePrefix      string `json:"invoice_prefix" validate:"required,lte=20"`
	Language           string `json:"language" validate:"omitempty,oneof=en id"`
	ReminderEnabled    bool   `json:"reminder_enabled"`
	ReminderBeforeDays int    `json:"reminder_before_days" validate:"gte=0,lte=365"`
	ReminderAfterDays  int    `json:"reminder_after_days" validate:"gte=0,lte=365"`
}

// EffectiveProviderMethods resolves the settings method value, preferring the
// canonical provider_methods key and falling back to the deprecated alias.
func (in SettingsInput) EffectiveProviderMethods() string {
	if in.ProviderMethods != "" {
		return in.ProviderMethods
	}
	return in.MidtransMethods
}
