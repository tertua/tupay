package database

import "gorm.io/gorm"

// backfillClientStatus is the forward data half of version 26: AutoMigrate adds
// the client status column with a default, but rows created before the column
// can carry NULL or '' on dialects that do not apply the default retroactively.
// This idempotent UPDATE stamps them active so every client has a concrete
// lifecycle status; the SQL is standard on both SQLite and PostgreSQL.
func backfillClientStatus(db *gorm.DB) error {
	return db.Exec(
		"UPDATE clients SET status = ? WHERE status IS NULL OR status = ''",
		"active",
	).Error
}
