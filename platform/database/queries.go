package database

import (
	"github.com/tertua/tupay/app/queries"
	"gorm.io/gorm"
)

// Queries struct for collect all app queries.
type Queries struct {
	*queries.UserQueries            // load queries from User model
	*queries.UserIdentityQueries    // load queries from UserIdentity model (OIDC SSO)
	*queries.ClientQueries          // load queries from Client model
	*queries.InvoiceQueries         // load queries from Invoice model
	*queries.ClientPortalQueries    // load queries for the public client portal
	*queries.ItemQueries            // load queries from Item model
	*queries.ExpenseQueries         // load queries from Expense model
	*queries.PaymentQueries         // load queries from Payment model
	*queries.GatewayQueries         // load queries for central payment relay
	*queries.ReportQueries          // load queries for Reports aggregates
	*queries.SettingsQueries        // load queries from Settings model
	*queries.DashboardQueries       // load queries for Dashboard aggregates
	*queries.IdempotencyQueries     // load queries for idempotency keys
	*queries.MailOutboxQueries      // load queries for mail outbox
	*queries.NotificationQueries    // load queries for notification webhooks
	*queries.InvoiceReminderQueries // load queries for automatic payment reminders
	*queries.AuditQueries           // load queries for audit trail
	*queries.OrgQueries             // load queries for organizations
	*queries.MembershipQueries      // load queries for org memberships
	*queries.OrgInviteQueries       // load queries for org invites
}

// newQueries wires every domain query struct onto the shared database handle.
func newQueries(db *gorm.DB) *Queries {
	return &Queries{
		UserQueries:            &queries.UserQueries{DB: db},            // from User model
		UserIdentityQueries:    &queries.UserIdentityQueries{DB: db},    // from UserIdentity model (OIDC SSO)
		ClientQueries:          &queries.ClientQueries{DB: db},          // from Client model
		InvoiceQueries:         &queries.InvoiceQueries{DB: db},         // from Invoice model
		ClientPortalQueries:    &queries.ClientPortalQueries{DB: db},    // public client portal
		ItemQueries:            &queries.ItemQueries{DB: db},            // from Item model
		ExpenseQueries:         &queries.ExpenseQueries{DB: db},         // from Expense model
		PaymentQueries:         &queries.PaymentQueries{DB: db},         // from Payment model
		GatewayQueries:         &queries.GatewayQueries{DB: db},         // for central payment relay
		ReportQueries:          &queries.ReportQueries{DB: db},          // for Reports aggregates
		SettingsQueries:        &queries.SettingsQueries{DB: db},        // from Settings model
		DashboardQueries:       &queries.DashboardQueries{DB: db},       // for Dashboard aggregates
		IdempotencyQueries:     &queries.IdempotencyQueries{DB: db},     // for idempotency keys
		MailOutboxQueries:      &queries.MailOutboxQueries{DB: db},      // for mail outbox
		NotificationQueries:    &queries.NotificationQueries{DB: db},    // for notification webhooks
		InvoiceReminderQueries: &queries.InvoiceReminderQueries{DB: db}, // for automatic payment reminders
		AuditQueries:           &queries.AuditQueries{DB: db},           // for audit trail
		OrgQueries:             &queries.OrgQueries{DB: db},             // for organizations
		MembershipQueries:      &queries.MembershipQueries{DB: db},      // for org memberships
		OrgInviteQueries:       &queries.OrgInviteQueries{DB: db},       // for org invites
	}
}
