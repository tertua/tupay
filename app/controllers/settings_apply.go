package controllers

import "github.com/tertua/tupay/app/models"

// applySettingsInput copies the editable fields onto settings. Language is only
// overwritten with a known locale so an empty payload never resets it. The
// usd_to_idr rate is validated by the caller before this runs.
func applySettingsInput(settings *models.Settings, input *models.SettingsInput) {
	settings.CompanyName = input.CompanyName
	settings.Email = input.Email
	settings.Phone = input.Phone
	settings.Address = input.Address
	settings.LogoURL = input.LogoURL
	settings.Currency = input.Currency
	settings.TaxRate = input.TaxRate
	settings.UsdToIdr = input.UsdToIdr
	settings.ProviderMethods = input.ProviderMethods
	settings.InvoicePrefix = input.InvoicePrefix
	settings.ReminderEnabled = input.ReminderEnabled
	settings.ReminderBeforeDays = input.ReminderBeforeDays
	settings.ReminderAfterDays = input.ReminderAfterDays
	if input.Language == "en" || input.Language == "id" {
		settings.Language = input.Language
	}
}
