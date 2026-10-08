package database

import (
	"path/filepath"
	"testing"

	"github.com/tertua/tupay/app/models"
)

// legacyInvoiceTemplateModel mirrors the pre-v25 header table for the test's
// simulated old database. It is declared here (not shared with the migration's
// legacy structs, which carry no columns) so AutoMigrate builds the real shape.
type legacyInvoiceTemplateModel struct {
	ID        string `gorm:"primaryKey"`
	Name      string
	Status    string
	Cadence   string
	OrgID     string
	CreatedAt int64
}

func (legacyInvoiceTemplateModel) TableName() string { return "invoice_templates" }

// legacySubscriptionRunModel mirrors the pre-v25 claim table, including the
// original unique index name (idx_invoice_template_run) that the migration must
// rename.
type legacySubscriptionRunModel struct {
	ID             string `gorm:"primaryKey"`
	OrgID          string
	SubscriptionID string `gorm:"uniqueIndex:idx_invoice_template_run"`
	RunDate        string `gorm:"uniqueIndex:idx_invoice_template_run"`
	CreatedAt      int64
}

func (legacySubscriptionRunModel) TableName() string { return "invoice_template_run" }

// TestSubscriptionRenameMigration proves SchemaVersion 25 renames the three
// recurring-template tables to their subscription names and renames the unique
// claim index, leaving NO lingering idx_invoice_template_run beside the new
// idx_subscription_run. It drives Migrate on one isolated SQLite database.
func TestSubscriptionRenameMigration(t *testing.T) {
	t.Setenv("SQL_DSN", "")
	t.Setenv("SQLITE_PATH", filepath.Join(t.TempDir(), "subscription_rename.db"))

	if err := Migrate(); err != nil {
		t.Fatalf("expected migrate to succeed, got: %v", err)
	}
	db, err := openShared()
	if err != nil {
		t.Fatalf("expected shared handle, got: %v", err)
	}

	// Simulate a database that predates the rename: drop the new tables and
	// recreate the old ones with the old index name inside the migration's
	// stamp, then reset the version so the next Migrate re-applies v25.
	if err := db.Migrator().DropTable(&models.SubscriptionRun{}, &models.SubscriptionItem{}, &models.Subscription{}); err != nil {
		t.Fatalf("expected to drop new tables for the simulation, got: %v", err)
	}
	if err := db.AutoMigrate(&legacyInvoiceTemplateModel{}, &legacySubscriptionRunModel{}); err != nil {
		t.Fatalf("expected to recreate pre-v25 tables, got: %v", err)
	}
	if !db.Migrator().HasTable("invoice_templates") {
		t.Fatal("expected invoice_templates before the rename")
	}
	// Seed a row so the test also proves the rename carries the data across.
	seededID := "11111111-1111-1111-1111-111111111111"
	if err := db.Exec("INSERT INTO invoice_templates (id, name, status) VALUES (?, ?, ?)",
		seededID, "Seeded Retainer", "active").Error; err != nil {
		t.Fatalf("expected to seed a pre-v25 row, got: %v", err)
	}

	// Stamp an older version so the forward step is exercised, then re-migrate.
	if err := db.Table("schema_migrations").Where("id = ?", 1).Update("version", 24).Error; err != nil {
		t.Fatalf("expected to stamp version 24, got: %v", err)
	}
	if err := Migrate(); err != nil {
		t.Fatalf("expected migrate to rename the tables, got: %v", err)
	}

	// New tables exist, old tables are gone.
	for _, name := range []string{"subscriptions", "subscription_items", "subscription_runs"} {
		if !db.Migrator().HasTable(name) {
			t.Errorf("expected table %q after the v25 rename", name)
		}
	}
	for _, name := range []string{"invoice_templates", "invoice_template_items", "invoice_template_run"} {
		if db.Migrator().HasTable(name) {
			t.Errorf("expected old table %q to be gone after the v25 rename", name)
		}
	}

	// The claim index must be renamed exactly once: the new name present, the
	// old name GONE (a lingering old index beside the new one is not allowed).
	if !db.Migrator().HasIndex(&models.SubscriptionRun{}, newSubscriptionRunIndex) {
		t.Errorf("expected index %q after the v25 rename", newSubscriptionRunIndex)
	}
	if db.Migrator().HasIndex(&models.SubscriptionRun{}, oldSubscriptionRunIndex) {
		t.Errorf("expected index %q to be renamed away, but it still exists", oldSubscriptionRunIndex)
	}

	// The seeded row rode the rename into the new table.
	var name string
	if err := db.Table("subscriptions").Where("id = ?", seededID).Pluck("name", &name).Error; err != nil || name != "Seeded Retainer" {
		t.Errorf("expected the seeded row to survive the rename in subscriptions, got %q (%v)", name, err)
	}

	if stamp, err := CurrentSchemaVersion(); err != nil || stamp != SchemaVersion {
		t.Fatalf("expected stamp %d after migrate, got %d (%v)", SchemaVersion, stamp, err)
	}
}

// TestSubscriptionRenameMigrationDown proves the v25 rollback renames the three
// tables and the claim index back, and that a forward re-migration restores the
// subscription names.
func TestSubscriptionRenameMigrationDown(t *testing.T) {
	t.Setenv("SQL_DSN", "")
	t.Setenv("SQLITE_PATH", filepath.Join(t.TempDir(), "subscription_rename_down.db"))

	if err := Migrate(); err != nil {
		t.Fatalf("expected migrate to succeed, got: %v", err)
	}
	db, err := openShared()
	if err != nil {
		t.Fatalf("expected shared handle, got: %v", err)
	}

	if _, err := MigrateDownTo(24); err != nil {
		t.Fatalf("expected rollback to 24, got: %v", err)
	}
	for _, name := range []string{"invoice_templates", "invoice_template_items", "invoice_template_run"} {
		if !db.Migrator().HasTable(name) {
			t.Errorf("expected old table %q restored after the v25 down", name)
		}
	}
	for _, name := range []string{"subscriptions", "subscription_items", "subscription_runs"} {
		if db.Migrator().HasTable(name) {
			t.Errorf("expected table %q to be gone after the v25 down", name)
		}
	}
	if !db.Migrator().HasIndex(&legacySubscriptionRunModel{}, oldSubscriptionRunIndex) {
		t.Errorf("expected index %q restored after the v25 down", oldSubscriptionRunIndex)
	}
	if db.Migrator().HasIndex(&legacySubscriptionRunModel{}, newSubscriptionRunIndex) {
		t.Errorf("expected index %q to be renamed away by the v25 down", newSubscriptionRunIndex)
	}

	// Forward re-applies the rename on the next startup.
	if err := Migrate(); err != nil {
		t.Fatalf("expected re-migrate to succeed, got: %v", err)
	}
	if !db.Migrator().HasTable(&models.Subscription{}) {
		t.Error("expected subscriptions table restored after re-migrate")
	}
	if db.Migrator().HasIndex(&models.SubscriptionRun{}, oldSubscriptionRunIndex) {
		t.Error("expected no lingering old index after re-migrate")
	}
}
