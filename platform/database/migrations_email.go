package database

import (
	"fmt"
	"strings"

	"github.com/tertua/tupay/app/models"
	"gorm.io/gorm"
)

// userEmailIndexName is the unique index over users.email. It is created by the
// explicit startup step below rather than a model tag so databases that still
// carry case-insensitive duplicates can be healed before the constraint lands:
// AutoMigrate runs before the startup steps and would otherwise try to build the
// unique index on the raw data and abort startup.
const userEmailIndexName = "idx_users_email"

// normalizeUserEmails lowercases and trims every user email, then refuses to
// continue when the normalization collapses two accounts onto the same address.
// It is idempotent (a clean table is a no-op) and uses only standard SQL so
// SQLite and PostgreSQL behave identically.
//
// Duplicates are never merged or deleted automatically: the whole startup
// aborts with a message naming the colliding addresses so an operator can
// resolve the data loss question by hand.
func normalizeUserEmails(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.User{}) {
		return nil
	}

	if err := db.Exec(
		"UPDATE users SET email = LOWER(TRIM(email)) WHERE email <> LOWER(TRIM(email))",
	).Error; err != nil {
		return err
	}

	// GROUP BY on the normalized address surfaces any collision the UPDATE
	// just created; HAVING > 1 keeps only the offending groups.
	rows, err := db.Raw(
		"SELECT LOWER(TRIM(email)) AS email FROM users GROUP BY LOWER(TRIM(email)) HAVING COUNT(*) > 1",
	).Rows()
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	var duplicates []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return err
		}
		duplicates = append(duplicates, email)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(duplicates) > 0 {
		return fmt.Errorf(
			"users.email has case-insensitive duplicates after normalization: %s — merge these accounts manually before starting",
			strings.Join(duplicates, ", "))
	}
	return nil
}

// ensureUserEmailIndex creates the unique users.email index when it is missing.
// The step runs after normalizeUserEmails, so by the time it fires the column is
// unique and the index build cannot fail on legacy data. New installs get the
// same index from the same call.
//
// The index is built with raw CREATE UNIQUE INDEX IF NOT EXISTS (standard on
// both SQLite and PostgreSQL) rather than Migrator().CreateIndex, because GORM's
// CreateIndex resolves the column list from the model's index tags and the model
// deliberately declares no uniqueIndex tag (see plan D1).
func ensureUserEmailIndex(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.User{}) {
		return nil
	}
	if db.Migrator().HasIndex(&models.User{}, userEmailIndexName) {
		return nil
	}
	return db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS " + userEmailIndexName + " ON users (email)").Error
}
