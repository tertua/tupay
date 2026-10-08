package database

import (
	"fmt"

	"github.com/tertua/tupay/app/models"
	"gorm.io/gorm"
)

// runStartupDataSteps runs every idempotent startup step that must complete before
// the forward-only version guard stamps the schema: the personal-org backfill,
// the v17 provider-neutral column rename, the v18 admin_claims removal and the
// v21 users.email normalization + unique index. A failure aborts startup so a
// half-migrated database never reaches the version stamp.
func runStartupDataSteps(db *gorm.DB) error {
	if err := backfillOrganizations(db); err != nil {
		return fmt.Errorf("organization backfill: %w", err)
	}
	if err := migrateProviderColumnsUp(db); err != nil {
		return fmt.Errorf("provider column rename: %w", err)
	}
	if err := dropLegacyAdminClaims(db); err != nil {
		return fmt.Errorf("legacy admin_claims drop: %w", err)
	}
	// Normalization must precede the index: the unique index can only build once
	// the column is free of case-insensitive duplicates (or startup fails loudly).
	if err := normalizeUserEmails(db); err != nil {
		return fmt.Errorf("user email normalization: %w", err)
	}
	if err := ensureUserEmailIndex(db); err != nil {
		return fmt.Errorf("user email index: %w", err)
	}
	if err := backfillClientStatus(db); err != nil {
		return fmt.Errorf("client status backfill: %w", err)
	}
	return nil
}

// dropLegacyAdminClaims is the forward half of version 18: databases upgraded
// from the claim-based bootstrap still carry the singleton table, and AutoMigrate
// only ever adds. Idempotent on SQLite and PostgreSQL.
func dropLegacyAdminClaims(db *gorm.DB) error {
	if !db.Migrator().HasTable(&legacyAdminClaim{}) {
		return nil
	}
	return db.Migrator().DropTable(&legacyAdminClaim{})
}

// providerColumnRename describes one v17 column rename: the old provider-named
// column copied into the new provider-neutral one, then dropped. The model's
// db tag must already point at newCol (AutoMigrate creates it); oldCol survives
// from the previous schema until the copy finishes.
type providerColumnRename struct {
	table  string
	model  any
	oldCol string
	newCol string
}

// providerColumnRenames is the single source of truth for the v17 rename that
// both the forward copy and the registry Down step refer to. Table names use the
// GORM plural convention (pinned by migrations_org_test.go) and are hardcoded
// because the raw UPDATE cannot interpolate a model.
var providerColumnRenames = []providerColumnRename{
	{table: "gateway_transactions", model: &models.GatewayTransaction{}, oldCol: "midtrans_txn_id", newCol: "provider_txn_id"},
	{table: "gateway_transactions", model: &models.GatewayTransaction{}, oldCol: "snap_token", newCol: "provider_token"},
	{table: "settings", model: &models.Settings{}, oldCol: "midtrans_methods", newCol: "provider_methods"},
}

// migrateProviderColumnsUp copies each old provider-named column into its
// neutral replacement, then drops the old one. AutoMigrate only ADDS the new
// column (empty), so this is the data-moving half of the rename.
//
// Idempotent and crash-safe: it acts only when BOTH columns exist. A fresh
// install has only the new column (skip), a fully-migrated database has only
// the new column (skip), and a crash between the UPDATE and the drop leaves the
// old column present, so the next startup re-runs the copy harmlessly before
// dropping it. The UPDATE ... SET new = old statement is valid on both SQLite
// and PostgreSQL.
func migrateProviderColumnsUp(db *gorm.DB) error {
	for _, r := range providerColumnRenames {
		if !db.Migrator().HasColumn(r.model, r.oldCol) {
			continue
		}
		if !db.Migrator().HasColumn(r.model, r.newCol) {
			continue // neither shape matches this binary: leave it alone
		}
		if err := db.Exec("UPDATE " + r.table + " SET " + r.newCol + " = " + r.oldCol).Error; err != nil {
			return err
		}
		if err := db.Migrator().DropColumn(r.model, r.oldCol); err != nil {
			return err
		}
	}
	return nil
}

// migrateProviderColumnsDown is the registry v17 rollback: rename each neutral
// column back to its provider-named predecessor so an older binary can read the
// schema. GORM's RenameColumn emits a standard ALTER TABLE ... RENAME COLUMN,
// which carries the data with it on both SQLite and PostgreSQL. HasColumn guards
// mirror the v16 Down style and keep the step idempotent.
func migrateProviderColumnsDown(db *gorm.DB) error {
	for _, r := range providerColumnRenames {
		if !db.Migrator().HasColumn(r.model, r.newCol) {
			continue
		}
		if err := db.Migrator().RenameColumn(r.model, r.newCol, r.oldCol); err != nil {
			return err
		}
	}
	return nil
}
