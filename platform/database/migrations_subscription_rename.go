package database

import (
	"fmt"

	"github.com/tertua/tupay/app/models"
	"gorm.io/gorm"
)

// subscriptionTableRename maps one v25 rename: the pre-rename table name (as a
// GORM model pinned to its old TableName) onto the model whose new TableName is
// the target. Table names follow the GORM plural convention; the old names are
// the custom singular/plural names the recurring-template feature shipped with.
type subscriptionTableRename struct {
	old    any
	target any
	oldTbl string
	newTbl string
}

// subscriptionTableRenames is the single source of truth for the v25 table
// rename that both the forward step and the registry Down step refer to. The
// old names are hardcoded because the raw identifiers are passed to the
// Migrator directly.
var subscriptionTableRenames = []subscriptionTableRename{
	{old: &legacyInvoiceTemplate{}, target: &models.Subscription{}, oldTbl: "invoice_templates", newTbl: "subscriptions"},
	{old: &legacyInvoiceTemplateItem{}, target: &models.SubscriptionItem{}, oldTbl: "invoice_template_items", newTbl: "subscription_items"},
	{old: &legacyInvoiceTemplateRun{}, target: &models.SubscriptionRun{}, oldTbl: "invoice_template_run", newTbl: "subscription_runs"},
}

// legacyInvoiceTemplate pins the recurring-template header to its pre-v25 table
// name so the forward step and the Down rollback can address the old table by
// model instead of raw SQL.
type legacyInvoiceTemplate struct{}

func (legacyInvoiceTemplate) TableName() string { return "invoice_templates" }

type legacyInvoiceTemplateItem struct{}

func (legacyInvoiceTemplateItem) TableName() string { return "invoice_template_items" }

type legacyInvoiceTemplateRun struct{}

func (legacyInvoiceTemplateRun) TableName() string { return "invoice_template_run" }

// migrateSchema applies the pre-AutoMigrate renames then AutoMigrate. The
// rename MUST run first: this binary's models already declare the new
// subscription table names, so AutoMigrate would otherwise CREATE empty
// subscription tables beside the still-present recurring-template tables and
// build the new claim index while the old one lingers.
func migrateSchema(db *gorm.DB) error {
	if err := renameSubscriptionTablesUp(db); err != nil {
		return err
	}
	return db.AutoMigrate(autoMigrateList()...)
}

// renameSubscriptionTablesUp is the forward half of version 25: it renames the
// three recurring-template tables to their subscription names, then renames the
// unique claim index. It MUST run before AutoMigrate on the same connection,
// because this binary's models already declare the new table names and
// AutoMigrate would otherwise CREATE empty subscription tables alongside the
// still-present old ones (and build the new index while the old one lingers).
//
// Idempotent and crash-safe: each table is renamed only when the old name
// exists and the new name does not. A fresh install has neither name (skip); a
// fully-migrated database has only the new name (skip); a crash between the two
// ALTERs leaves the old name present, so the next startup finishes the rename.
// indexRename runs only after the table it belongs to is at its new name.
func renameSubscriptionTablesUp(db *gorm.DB) error {
	for _, r := range subscriptionTableRenames {
		if !db.Migrator().HasTable(r.oldTbl) {
			continue // nothing to rename (fresh install or already migrated)
		}
		if db.Migrator().HasTable(r.newTbl) {
			continue // both shapes present: this binary never caused that; leave it alone
		}
		if err := db.Migrator().RenameTable(r.old, r.target); err != nil {
			return fmt.Errorf("rename %s to %s: %w", r.oldTbl, r.newTbl, err)
		}
	}
	return renameSubscriptionRunIndexUp(db)
}

// subscriptionRunIndex is the unique (subscription_id, run_date) claim index.
// The old name predates the rename; the new name matches the model's
// uniqueIndex tag so AutoMigrate finds it already present.
const (
	oldSubscriptionRunIndex = "idx_invoice_template_run"
	newSubscriptionRunIndex = "idx_subscription_run"
)

// renameSubscriptionRunIndexUp renames the claim index on the (now renamed)
// subscription_runs table. GORM's Migrator.RenameIndex is dialect-aware: it
// emits ALTER INDEX ... RENAME on PostgreSQL and a drop-and-recreate on SQLite,
// so no raw SQL is needed here. It acts only when the table is at its new name,
// the old index exists, and the new index does not.
func renameSubscriptionRunIndexUp(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.SubscriptionRun{}) {
		return nil
	}
	if !db.Migrator().HasIndex(&models.SubscriptionRun{}, oldSubscriptionRunIndex) {
		return nil
	}
	if db.Migrator().HasIndex(&models.SubscriptionRun{}, newSubscriptionRunIndex) {
		return nil
	}
	return db.Migrator().RenameIndex(&models.SubscriptionRun{}, oldSubscriptionRunIndex, newSubscriptionRunIndex)
}

// renameSubscriptionTablesDown is the registry v25 rollback: it renames the
// subscription tables and the claim index back to the recurring-template names
// so an older binary can read the schema. GORM's Migrator.RenameTable carries
// the data with it on both SQLite and PostgreSQL; HasTable/HasIndex guards keep
// the step idempotent and mirror the v16/v17 Down style.
func renameSubscriptionTablesDown(db *gorm.DB) error {
	if err := renameSubscriptionRunIndexDown(db); err != nil {
		return err
	}
	for _, r := range subscriptionTableRenames {
		if !db.Migrator().HasTable(r.newTbl) {
			continue
		}
		if db.Migrator().HasTable(r.oldTbl) {
			continue // a half-rolled-back database that already restored the old name
		}
		if err := db.Migrator().RenameTable(r.target, r.old); err != nil {
			return fmt.Errorf("rename %s back to %s: %w", r.newTbl, r.oldTbl, err)
		}
	}
	return nil
}

// renameSubscriptionRunIndexDown reverses the claim index rename. It runs before
// the table rename so the index still lives on the old table when the migrator
// resolves it.
func renameSubscriptionRunIndexDown(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.SubscriptionRun{}) {
		return nil
	}
	if !db.Migrator().HasIndex(&models.SubscriptionRun{}, newSubscriptionRunIndex) {
		return nil
	}
	if db.Migrator().HasIndex(&models.SubscriptionRun{}, oldSubscriptionRunIndex) {
		return nil
	}
	return db.Migrator().RenameIndex(&models.SubscriptionRun{}, newSubscriptionRunIndex, oldSubscriptionRunIndex)
}
