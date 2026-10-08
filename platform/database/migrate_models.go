package database

import "github.com/tertua/tupay/app/models"

// autoMigrateList returns every model GORM creates or updates at startup.
// It lives here (not in open_db_connection.go) to keep that file under its
// size baseline after new models are added.
func autoMigrateList() []any {
	return []any{
		&models.User{}, &models.UserIdentity{},
		&models.Client{},
		&models.Invoice{},
		&models.InvoiceItem{},
		&models.Item{},
		&models.Expense{},
		&models.Payment{},
		&models.PaymentLink{},
		&models.ClientLink{},
		&models.GatewayProject{},
		&models.GatewayTransaction{},
		&models.GatewayEvent{},
		&models.WebhookDelivery{},
		&models.Settings{},
		&models.PasswordReset{},
		&models.EmailVerification{},
		&models.IdempotencyKey{},
		&models.MailOutbox{},
		&models.NotificationEndpoint{},
		&models.NotificationDelivery{},
		&models.SchemaMigration{},
		&models.AuditLog{},
		&models.Organization{},
		&models.Membership{},
		&models.OrgInvite{},
		&models.InvoiceReminderLog{},
		&models.Subscription{},
		&models.SubscriptionItem{},
		&models.SubscriptionRun{},
	}
}
