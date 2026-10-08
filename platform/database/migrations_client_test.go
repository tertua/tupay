package database

import (
	"path/filepath"
	"testing"

	"github.com/tertua/tupay/app/models"
)

// TestClientLinksMigration proves SchemaVersion 23 lands the client_links
// table with its client_id unique index and that a re-migrate is idempotent.
func TestClientLinksMigration(t *testing.T) {
	t.Setenv("SQL_DSN", "")
	t.Setenv("SQLITE_PATH", filepath.Join(t.TempDir(), "client_links.db"))

	if err := Migrate(); err != nil {
		t.Fatalf("expected migrate to succeed, got: %v", err)
	}
	db, err := openShared()
	if err != nil {
		t.Fatalf("expected shared handle, got: %v", err)
	}
	if !db.Migrator().HasTable(&models.ClientLink{}) {
		t.Fatal("expected client_links table after migrate")
	}
	if !db.Migrator().HasIndex(&models.ClientLink{}, "ClientID") {
		t.Fatal("expected unique index on client_id after migrate")
	}

	stamp, err := CurrentSchemaVersion()
	if err != nil {
		t.Fatalf("expected version stamp, got: %v", err)
	}
	if stamp != SchemaVersion {
		t.Fatalf("expected schema version %d, got %d", SchemaVersion, stamp)
	}

	// Re-migrate must be a no-op (idempotent) and keep the table.
	if err := Migrate(); err != nil {
		t.Fatalf("expected re-migrate to succeed, got: %v", err)
	}
	if !db.Migrator().HasTable(&models.ClientLink{}) {
		t.Error("expected client_links table to survive re-migrate")
	}
}

// TestClientLinksMigrationDown proves the v23 rollback drops the fresh table
// and the forward migrate recreates it.
func TestClientLinksMigrationDown(t *testing.T) {
	t.Setenv("SQL_DSN", "")
	t.Setenv("SQLITE_PATH", filepath.Join(t.TempDir(), "client_links_down.db"))

	if err := Migrate(); err != nil {
		t.Fatalf("expected migrate to succeed, got: %v", err)
	}
	db, err := openShared()
	if err != nil {
		t.Fatalf("expected shared handle, got: %v", err)
	}

	ver, err := MigrateDownTo(22)
	if err != nil {
		t.Fatalf("expected rollback to 22, got: %v", err)
	}
	if ver != 22 {
		t.Fatalf("expected returned version 22, got %d", ver)
	}
	if db.Migrator().HasTable(&models.ClientLink{}) {
		t.Error("expected client_links table dropped after rollback")
	}

	// Forward re-applies automatically on next startup.
	if err := Migrate(); err != nil {
		t.Fatalf("expected re-migrate to succeed, got: %v", err)
	}
	if !db.Migrator().HasTable(&models.ClientLink{}) {
		t.Error("expected client_links table restored after re-migrate")
	}
	if stamp, err := CurrentSchemaVersion(); err != nil || stamp != SchemaVersion {
		t.Fatalf("expected stamp %d after re-migrate, got %d (%v)", SchemaVersion, stamp, err)
	}
}
