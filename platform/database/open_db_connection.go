package database

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/configs"
	"gorm.io/gorm"
)

var (
	sharedDB  *gorm.DB
	sharedErr error
	dbOnce    sync.Once
)

// openShared opens the database handle once per process.
func openShared() (*gorm.DB, error) {
	dbOnce.Do(func() {
		sharedDB, sharedErr = chooseDB(configs.Get().DSN())
	})
	return sharedDB, sharedErr
}

// OpenDBConnection func for opening database connection.
func OpenDBConnection() (*Queries, error) {
	db, err := openShared()
	if err != nil {
		return nil, err
	}

	return newQueries(db), nil
}

// SchemaVersion is the current schema revision; bump it by 1 whenever a model changes so the version guard below can detect newer databases.
const SchemaVersion = 22

// Migrate creates or updates tables from models, then enforces the forward-only version guard (newer DB than binary is fatal).
func Migrate() error {
	db, err := openShared()
	if err != nil {
		return err
	}

	if err := db.AutoMigrate(autoMigrateList()...); err != nil {
		return err
	}

	// Data heal (idempotent, no schema change): re-mark invoices fully covered by non-voided payments as paid so the status column stays in sync for rows created before auto-marking; the SQL is standard on both SQLite and PostgreSQL.
	_ = db.Exec(
		"UPDATE invoices SET status = 'paid', updated_at = ? "+
			"WHERE status = 'sent' AND total > 0 AND "+
			"(SELECT COALESCE(SUM(amount), 0) FROM payments "+
			"WHERE payments.invoice_id = invoices.id AND payments.voided_at IS NULL) >= invoices.total",
		time.Now(),
	).Error

	// Idempotent startup data steps (org backfill + v17 provider column rename) must finish before the version guard: any failure aborts startup.
	if err := runStartupDataSteps(db); err != nil {
		return err
	}

	return checkSchemaVersion(db)
}

// checkSchemaVersion implements the forward-only guard.
func checkSchemaVersion(db *gorm.DB) error {
	row := models.SchemaMigration{}
	err := db.Where("id = ?", 1).First(&row).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// Fresh database: stamp it.
		return db.Create(&models.SchemaMigration{
			ID: 1, Version: SchemaVersion,
			AppVersion: appVersion(), AppliedAt: time.Now(),
		}).Error
	}
	if row.Version > SchemaVersion {
		return fmt.Errorf(
			"database schema version %d is newer than this binary (version %d): refusing to start",
			row.Version, SchemaVersion)
	}
	if row.Version < SchemaVersion {
		row.Version = SchemaVersion
		row.AppVersion = appVersion()
		row.AppliedAt = time.Now()
		return db.Save(&row).Error
	}
	return nil
}

// appVersion reads the single-source VERSION file, falling back to "dev".
func appVersion() string {
	raw, err := os.ReadFile("VERSION")
	if err != nil {
		return "dev"
	}
	if v := strings.TrimSpace(string(raw)); v != "" {
		return v
	}
	return "dev"
}
