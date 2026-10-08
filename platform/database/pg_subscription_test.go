package database

import (
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tertua/tupay/app/models"
)

// TestPostgresSubscriptionTables validates the subscription DDL on a real
// PostgreSQL server: AutoMigrate for the three subscription tables, a CRUD
// round trip, and the unique (subscription_id, run_date) claim index that makes
// the sweep idempotent. It runs only when INVOICEMAN_TEST_PG_DSN points at a
// SCRATCH database; otherwise it skips, like TestPostgresBackend.
func TestPostgresSubscriptionTables(t *testing.T) {
	dsn := os.Getenv("INVOICEMAN_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set INVOICEMAN_TEST_PG_DSN to a scratch PostgreSQL database to run")
	}

	db, err := chooseDB(dsn)
	if err != nil {
		t.Fatalf("expected PostgreSQL handle, got: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("expected sql handle, got: %v", err)
	}
	defer sqlDB.Close()

	if err := db.AutoMigrate(
		&models.Subscription{},
		&models.SubscriptionItem{},
		&models.SubscriptionRun{},
	); err != nil {
		t.Fatalf("expected subscription automigrate on PostgreSQL, got: %v", err)
	}

	orgID := uuid.New()
	sub := &models.Subscription{
		ID: uuid.New(), CreatedAt: time.Now(), UserID: uuid.New(), OrgID: orgID,
		Name: "PG Smoke Retainer", Status: models.SubscriptionStatusActive,
		Cadence:  models.SubscriptionCadenceWeekly,
		Currency: "IDR", Discount: models.ZeroMoney, InvoiceStatus: "draft",
	}
	if err := db.Create(sub).Error; err != nil {
		t.Fatalf("expected subscription create on PostgreSQL, got: %v", err)
	}
	item := &models.SubscriptionItem{
		ID: uuid.New(), SubscriptionID: sub.ID,
		Description: "Monthly retainer", Quantity: 1, Rate: models.ZeroMoney,
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("expected subscription item create on PostgreSQL, got: %v", err)
	}
	runDate := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	run := &models.SubscriptionRun{
		ID: uuid.New(), OrgID: orgID, SubscriptionID: sub.ID,
		RunDate: runDate, CreatedAt: time.Now(),
	}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("expected run claim on PostgreSQL, got: %v", err)
	}
	// The claim index must reject a second claim for the same occurrence.
	dup := &models.SubscriptionRun{
		ID: uuid.New(), OrgID: orgID, SubscriptionID: sub.ID,
		RunDate: runDate, CreatedAt: time.Now(),
	}
	if err := db.Create(dup).Error; err == nil {
		t.Fatal("expected duplicate run claim to fail on PostgreSQL")
	}

	var got models.Subscription
	if err := db.First(&got, "id = ?", sub.ID).Error; err != nil || got.Name != sub.Name {
		t.Fatalf("expected subscription read back on PostgreSQL, got %+v (%v)", got, err)
	}

	for _, stmt := range []struct {
		model any
		id    uuid.UUID
	}{
		{&models.SubscriptionRun{}, run.ID},
		{&models.SubscriptionItem{}, item.ID},
		{&models.Subscription{}, sub.ID},
	} {
		if err := db.Delete(stmt.model, "id = ?", stmt.id).Error; err != nil {
			t.Fatalf("expected cleanup delete, got: %v", err)
		}
	}
}
