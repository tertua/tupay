package database

import (
	"time"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"gorm.io/gorm"
)

// legacyAdminClaim maps the admin_claims table for schema history only: the
// singleton first-admin bootstrap (version 3) was removed in version 18, so no
// model in app/models maps this table anymore.
type legacyAdminClaim struct {
	ID        int       `gorm:"primaryKey" db:"id"`
	UserID    uuid.UUID `gorm:"type:uuid" db:"user_id"`
	CreatedAt time.Time `db:"created_at"`
}

// TableName keeps the plural convention used by other models.
func (legacyAdminClaim) TableName() string { return "admin_claims" }

// migrations lists every rollback known to this binary, oldest first.
// Ups run implicitly through AutoMigrate at startup; only the Down
// rollback is recorded here, keyed by the SchemaVersion that introduced
// the change.
var migrations = []Migration{
	{
		Version:     2,
		Description: "expense receipt_url + mail_outbox html_body",
		Down: func(db *gorm.DB) error {
			if err := db.Migrator().DropColumn(&models.Expense{}, "receipt_url"); err != nil {
				return err
			}
			return db.Migrator().DropColumn(&models.MailOutbox{}, "html_body")
		},
	},
	{
		Version:     3,
		Description: "admin_claims singleton for atomic first-admin bootstrap",
		Down: func(db *gorm.DB) error {
			// DROP TABLE IF EXISTS on both backends: databases created after
			// version 18 never carry the table.
			return db.Migrator().DropTable(&legacyAdminClaim{})
		},
	},
	{
		Version:     4,
		Description: "idempotent local gateway payment settlement",
		Down: func(db *gorm.DB) error {
			return db.Migrator().DropColumn(&models.Payment{}, "gateway_order_id")
		},
	},
	{
		Version:     5,
		Description: "notification endpoints + deliveries",
		Down: func(db *gorm.DB) error {
			if err := db.Migrator().DropTable(&models.NotificationDelivery{}); err != nil {
				return err
			}
			return db.Migrator().DropTable(&models.NotificationEndpoint{})
		},
	},
	{
		Version:     6,
		Description: "payment void columns (voided_at + void_reason)",
		Down: func(db *gorm.DB) error {
			if err := db.Migrator().DropColumn(&models.Payment{}, "voided_at"); err != nil {
				return err
			}
			return db.Migrator().DropColumn(&models.Payment{}, "void_reason")
		},
	},
	{
		Version:     7,
		Description: "settings language (per-user UI locale)",
		Down: func(db *gorm.DB) error {
			return db.Migrator().DropColumn(&models.Settings{}, "language")
		},
	},
	{
		Version:     8,
		Description: "fixed-point decimal money columns",
		Down: func(db *gorm.DB) error {
			// Money fields are part of the base tables; fresh dev databases are
			// recreated for this schema change rather than backfilled in place.
			return nil
		},
	},
	{
		Version:     9,
		Description: "gateway project ownership and external invoice identities",
		Down: func(db *gorm.DB) error {
			if err := db.Migrator().DropColumn(&models.GatewayProject{}, "owner_user_id"); err != nil {
				return err
			}
			if err := db.Migrator().DropColumn(&models.Client{}, "gateway_project_slug"); err != nil {
				return err
			}
			if err := db.Migrator().DropColumn(&models.Client{}, "external_id"); err != nil {
				return err
			}
			if err := db.Migrator().DropColumn(&models.Invoice{}, "gateway_project_slug"); err != nil {
				return err
			}
			return db.Migrator().DropColumn(&models.Invoice{}, "external_id")
		},
	},
	{
		Version:     10,
		Description: "gateway transaction payment method",
		Down: func(db *gorm.DB) error {
			return db.Migrator().DropColumn(&models.GatewayTransaction{}, "payment_method")
		},
	},
	{
		Version:     11,
		Description: "manual USD/IDR gateway conversion",
		Down: func(db *gorm.DB) error {
			if err := db.Migrator().DropColumn(&models.Settings{}, "usd_to_idr"); err != nil {
				return err
			}
			if err := db.Migrator().DropColumn(&models.GatewayTransaction{}, "invoice_currency"); err != nil {
				return err
			}
			if err := db.Migrator().DropColumn(&models.GatewayTransaction{}, "invoice_amount"); err != nil {
				return err
			}
			return db.Migrator().DropColumn(&models.GatewayTransaction{}, "usd_to_idr")
		},
	},
	{
		Version:     12,
		Description: "per-account Midtrans payment method allowlist",
		Down: func(db *gorm.DB) error {
			return db.Migrator().DropColumn(&models.Settings{}, "midtrans_methods")
		},
	},
	{
		Version:     13,
		Description: "nowpayments direct payment fields (pay_amount, pay_currency, expires_at)",
		Down: func(db *gorm.DB) error {
			if err := db.Migrator().DropColumn(&models.GatewayTransaction{}, "pay_amount"); err != nil {
				return err
			}
			if err := db.Migrator().DropColumn(&models.GatewayTransaction{}, "pay_currency"); err != nil {
				return err
			}
			return db.Migrator().DropColumn(&models.GatewayTransaction{}, "expires_at")
		},
	},
	{
		Version:     14,
		Description: "invoice informational payment_method column",
		Down: func(db *gorm.DB) error {
			return db.Migrator().DropColumn(&models.Invoice{}, "payment_method")
		},
	},
	{
		Version:     15,
		Description: "gateway transaction qr_string (on-page QRIS payload)",
		Down: func(db *gorm.DB) error {
			return db.Migrator().DropColumn(&models.GatewayTransaction{}, "qr_string")
		},
	},
	{
		Version:     16,
		Description: "organizations + memberships + org_id tenancy (backfill)",
		Down: func(db *gorm.DB) error {
			// settings.org_id is rebuilt instead of dropped in place because the SQLite migrator keeps PRIMARY KEY (org_id) after the column goes away.
			if err := dropSettingsOrgID(db); err != nil {
				return err
			}
			// HasColumn guards skip columns this binary never added.
			for _, target := range []struct {
				model any
				col   string
			}{
				{&models.Invoice{}, "org_id"},
				{&models.Client{}, "org_id"},
				{&models.Item{}, "org_id"},
				{&models.Expense{}, "org_id"},
				{&models.Payment{}, "org_id"},
				{&models.GatewayProject{}, "org_id"},
				{&models.AuditLog{}, "org_id"},
			} {
				if !db.Migrator().HasColumn(target.model, target.col) {
					continue
				}
				if err := db.Migrator().DropColumn(target.model, target.col); err != nil {
					return err
				}
			}
			for _, model := range []any{&models.OrgInvite{}, &models.Membership{}, &models.Organization{}} {
				if err := db.Migrator().DropTable(model); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:     17,
		Description: "provider-neutral gateway columns (provider_txn_id, provider_token, provider_methods)",
		// The forward rename (copy old -> new, then drop old) lives in
		// migrateProviderColumnsUp; only the rollback is registered here.
		Down: migrateProviderColumnsDown,
	},
	{
		Version:     18,
		Description: "remove admin_claims (first-admin falls out of the register count)",
		// Forward: dropLegacyAdminClaims at startup. Down restores the table so
		// a rollback to 17 leaves the schema the previous binary expects.
		Down: func(db *gorm.DB) error {
			return db.AutoMigrate(&legacyAdminClaim{})
		},
	},
	{
		Version:     19,
		Description: "email verification tokens for pending registrations",
		Down: func(db *gorm.DB) error {
			return db.Migrator().DropTable(&models.EmailVerification{})
		},
	},
	{
		Version:     20,
		Description: "OIDC user identities (provider, sub)",
		Down: func(db *gorm.DB) error {
			return db.Migrator().DropTable(&models.UserIdentity{})
		},
	},
	{
		Version:     21,
		Description: "unique users.email + normalized emails",
		// The forward half lives in the startup data steps: emails are trimmed
		// and lowercased, duplicates abort startup, then the unique index is
		// created. Down only drops the index (the normalized values stay).
		Down: func(db *gorm.DB) error {
			if !db.Migrator().HasIndex(&models.User{}, userEmailIndexName) {
				return nil
			}
			return db.Migrator().DropIndex(&models.User{}, userEmailIndexName)
		},
	},
	{
		Version:     22,
		Description: "automatic payment reminders (invoice_reminder_log + per-org schedule)",
		// Down drops the reminder ledger and the three per-org schedule columns.
		// AutoMigrate re-adds nothing on rollback; the next startup with this
		// binary re-applies them (forward upgrades run through AutoMigrate).
		Down: func(db *gorm.DB) error {
			if err := db.Migrator().DropTable(&models.InvoiceReminderLog{}); err != nil {
				return err
			}
			for _, col := range []string{"reminder_enabled", "reminder_before_days", "reminder_after_days"} {
				if !db.Migrator().HasColumn(&models.Settings{}, col) {
					continue
				}
				if err := db.Migrator().DropColumn(&models.Settings{}, col); err != nil {
					return err
				}
			}
			return nil
		},
	},
}
