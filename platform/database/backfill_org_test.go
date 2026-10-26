package database

import (
	"testing"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"gorm.io/gorm"
)

// TestBackfillStampsOwnerlessGatewayProject pins the tenant stamp for a legacy
// relay project registered before orgs existed and never given an owner.
//
// The project survives the migration with org_id = NULL, and the intent path
// then reads GetSettings(project.OrgID) — so a project without a tenant
// silently falls through to the nil org. Every project that outlives the
// backfill must carry an org.
func TestBackfillStampsOwnerlessGatewayProject(t *testing.T) {
	db := probeDB(t)
	probeMigrate(t, db)

	owner := models.User{ID: uuid.New(), Name: "Relay Owner", Email: "relay-owner@example.com", PasswordHash: "hash", UserStatus: 1, UserRole: "user"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatalf("expected probe user, got: %v", err)
	}

	// Legacy shape: a project with no owner_user_id and no org_id at all.
	legacy := models.GatewayProject{
		Slug: "legacy-relay", Name: "Legacy Relay", WebhookURL: "https://relay.example.com/hook",
		APIKeyHash: "legacy-hash", WebhookSecret: "secret", DefaultGateway: "midtrans", IsActive: true,
	}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatalf("expected legacy project, got: %v", err)
	}

	if err := backfillOrganizations(db); err != nil {
		t.Fatalf("expected backfill to succeed, got: %v", err)
	}

	var project models.GatewayProject
	if err := db.Where("slug = ?", legacy.Slug).First(&project).Error; err != nil {
		t.Fatalf("expected project to survive the backfill, got: %v", err)
	}
	if project.OrgID == uuid.Nil {
		t.Fatalf("expected the ownerless project to be stamped with an org, got nil")
	}
	// The stamp must point at a real org so settings and invoices resolve.
	var org models.Organization
	if err := db.Where("id = ?", project.OrgID).First(&org).Error; err != nil {
		t.Fatalf("expected the stamped org to exist, got: %v", err)
	}

	// A second run must reuse that org for the project, not mint another one.
	if err := backfillOrganizations(db); err != nil {
		t.Fatalf("expected the second backfill to succeed, got: %v", err)
	}
	var after models.GatewayProject
	if err := db.Where("slug = ?", legacy.Slug).First(&after).Error; err != nil {
		t.Fatalf("expected project after the second backfill, got: %v", err)
	}
	if after.OrgID != project.OrgID {
		t.Errorf("expected the project to keep org %s, got %s", project.OrgID, after.OrgID)
	}
	var systemOrgs int64
	if err := db.Model(&models.Organization{}).Where("name = ?", systemOrgName).Count(&systemOrgs).Error; err != nil {
		t.Fatalf("expected to count system orgs, got: %v", err)
	}
	if systemOrgs != 1 {
		t.Errorf("expected one system org after two backfills, got %d", systemOrgs)
	}
}

// TestBackfillStampsGatewayProjectFromOwner proves the common case still holds:
// an owned project is stamped with the owner's org, not a second one.
func TestBackfillStampsGatewayProjectFromOwner(t *testing.T) {
	db := probeDB(t)
	probeMigrate(t, db)

	owner := models.User{ID: uuid.New(), Name: "Owned Relay", Email: "owned-relay@example.com", PasswordHash: "hash", UserStatus: 1, UserRole: "user"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatalf("expected probe user, got: %v", err)
	}
	project := models.GatewayProject{
		Slug: "owned-relay", Name: "Owned Relay", WebhookURL: "https://relay.example.com/hook",
		APIKeyHash: "owned-hash", WebhookSecret: "secret", DefaultGateway: "midtrans", IsActive: true,
		GatewayProjectOwner: models.GatewayProjectOwner{OwnerUserID: &owner.ID},
	}
	if err := db.Create(&project).Error; err != nil {
		t.Fatalf("expected owned project, got: %v", err)
	}

	if err := backfillOrganizations(db); err != nil {
		t.Fatalf("expected backfill to succeed, got: %v", err)
	}

	var stamped models.GatewayProject
	if err := db.Where("slug = ?", project.Slug).First(&stamped).Error; err != nil {
		t.Fatalf("expected project to survive, got: %v", err)
	}
	var membership models.Membership
	if err := db.Where("user_id = ?", owner.ID).First(&membership).Error; err != nil {
		t.Fatalf("expected a personal membership for the owner, got: %v", err)
	}
	if stamped.OrgID != membership.OrgID {
		t.Errorf("expected project org %s to be the owner's org %s", stamped.OrgID, membership.OrgID)
	}
}

// staffOnlyUser seeds a user whose only membership is staff in a foreign org, the shape an invited teammate has.
func staffOnlyUser(t *testing.T, db *gorm.DB, email string) (user models.User, hostOrgID uuid.UUID) {
	t.Helper()
	user = models.User{ID: uuid.New(), Name: "Invited Staff", Email: email, PasswordHash: "hash", UserStatus: 1, UserRole: "user"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("expected staff user, got: %v", err)
	}
	foreign := models.Organization{ID: uuid.New(), Name: "Host Org"}
	if err := db.Create(&foreign).Error; err != nil {
		t.Fatalf("expected host org, got: %v", err)
	}
	m := models.Membership{ID: uuid.New(), OrgID: foreign.ID, UserID: user.ID, Role: models.RoleStaff}
	if err := db.Create(&m).Error; err != nil {
		t.Fatalf("expected staff membership, got: %v", err)
	}
	return user, foreign.ID
}

// TestBackfillProvisionsOrgForStaffOnlyLegacyRows covers a staff-only user whose
// rows nobody else can stamp: they joined an org that predates their data, so a
// personal org is minted and their rows become visible instead of staying NULL.
func TestBackfillProvisionsOrgForStaffOnlyLegacyRows(t *testing.T) {
	db := probeDB(t)
	probeMigrate(t, db)

	user, foreignOrg := staffOnlyUser(t, db, "staff-legacy@example.com")
	client := models.Client{ID: uuid.New(), UserID: user.ID, Name: "Legacy Client"}
	if err := db.Create(&client).Error; err != nil {
		t.Fatalf("expected legacy client, got: %v", err)
	}

	if err := backfillOrganizations(db); err != nil {
		t.Fatalf("expected backfill to succeed, got: %v", err)
	}

	var ownerOrg models.Membership
	if err := db.Where("user_id = ? AND role = ?", user.ID, models.RoleOwner).First(&ownerOrg).Error; err != nil {
		t.Fatalf("expected a personal owner membership, got: %v", err)
	}
	if ownerOrg.OrgID == foreignOrg {
		t.Error("expected a personal org, not the foreign staff org")
	}
	var stamped models.Client
	if err := db.Where("id = ?", client.ID).First(&stamped).Error; err != nil {
		t.Fatalf("expected client to survive, got: %v", err)
	}
	if stamped.OrgID != ownerOrg.OrgID {
		t.Errorf("expected client stamped with the personal org %s, got %s", ownerOrg.OrgID, stamped.OrgID)
	}
}

// TestBackfillStaffOnlyLeavesOrglessWhenNoLegacyRows pins the gate: a staff-only
// user with nothing to stamp keeps no personal org (an invite must not spawn one).
func TestBackfillStaffOnlyLeavesOrglessWhenNoLegacyRows(t *testing.T) {
	db := probeDB(t)
	probeMigrate(t, db)

	user, _ := staffOnlyUser(t, db, "staff-clean@example.com")

	if err := backfillOrganizations(db); err != nil {
		t.Fatalf("expected backfill to succeed, got: %v", err)
	}

	var owners int64
	if err := db.Model(&models.Membership{}).Where("user_id = ? AND role = ?", user.ID, models.RoleOwner).Count(&owners).Error; err != nil {
		t.Fatalf("expected to count owner memberships, got: %v", err)
	}
	if owners != 0 {
		t.Errorf("expected no personal org for a clean staff-only user, got %d owner memberships", owners)
	}
}

// TestBackfillOwnerPathIdempotent proves a second run neither mints a second org
// nor changes the rows the first run already stamped.
func TestBackfillOwnerPathIdempotent(t *testing.T) {
	db := probeDB(t)
	probeMigrate(t, db)

	owner := models.User{ID: uuid.New(), Name: "Idem Owner", Email: "idem-owner@example.com", PasswordHash: "hash", UserStatus: 1, UserRole: "user"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatalf("expected owner, got: %v", err)
	}
	client := models.Client{ID: uuid.New(), UserID: owner.ID, Name: "Idem Client"}
	if err := db.Create(&client).Error; err != nil {
		t.Fatalf("expected client, got: %v", err)
	}

	if err := backfillOrganizations(db); err != nil {
		t.Fatalf("expected first backfill to succeed, got: %v", err)
	}
	var first models.Membership
	if err := db.Where("user_id = ? AND role = ?", owner.ID, models.RoleOwner).First(&first).Error; err != nil {
		t.Fatalf("expected personal membership, got: %v", err)
	}

	if err := backfillOrganizations(db); err != nil {
		t.Fatalf("expected second backfill to succeed, got: %v", err)
	}
	var orgs int64
	if err := db.Model(&models.Organization{}).Count(&orgs).Error; err != nil {
		t.Fatalf("expected to count orgs, got: %v", err)
	}
	if orgs != 1 {
		t.Errorf("expected exactly one org after two runs, got %d", orgs)
	}
	var after models.Client
	if err := db.Where("id = ?", client.ID).First(&after).Error; err != nil {
		t.Fatalf("expected client after second run, got: %v", err)
	}
	if after.OrgID != first.OrgID {
		t.Errorf("expected client to keep org %s, got %s", first.OrgID, after.OrgID)
	}
}
