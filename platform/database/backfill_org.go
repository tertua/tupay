package database

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"gorm.io/gorm"
)

// backfillOrganizations gives each legacy user a personal owner org and stamps org_id on their rows; idempotent, runs after AutoMigrate.
func backfillOrganizations(db *gorm.DB) error {
	var users []models.User
	if err := db.Find(&users).Error; err != nil {
		return err
	}
	for _, user := range users {
		orgID, ok, err := ownerOrgFor(db, user)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if err := stampOrgColumns(db, user.ID, orgID); err != nil {
			return err
		}
	}
	if err := stampOwnerlessProjects(db); err != nil {
		return err
	}
	return rebuildSettingsPK(db)
}

// systemOrgName labels the shared tenant that adopts relay projects which predate orgs and never had an owner to inherit one from.
const systemOrgName = "Relay (system)"

// stampOwnerlessProjects adopts relay projects the per-user pass left unstamped.
// A project with no owner_user_id has no user to derive a tenant from, but it
// still holds a live API key: the intent path reads settings by project.OrgID,
// so a NULL org would silently fall through to the nil tenant. They share one
// system org (created once, reused) that an admin can reassign later.
func stampOwnerlessProjects(db *gorm.DB) error {
	var pending []models.GatewayProject
	if err := db.Where("org_id IS NULL OR org_id = ?", uuid.Nil).Find(&pending).Error; err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}
	orgID, err := systemOrgID(db)
	if err != nil {
		return err
	}
	for _, project := range pending {
		if err := db.Model(&models.GatewayProject{}).Where("slug = ?", project.Slug).
			UpdateColumn("org_id", orgID).Error; err != nil {
			return err
		}
	}
	return nil
}

// systemOrgID returns the shared system org, creating it on first use; a later run reuses the existing row instead of minting another.
func systemOrgID(db *gorm.DB) (uuid.UUID, error) {
	var org models.Organization
	if err := db.Where("name = ?", systemOrgName).First(&org).Error; err == nil {
		return org.ID, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return uuid.Nil, err
	}
	org = models.Organization{ID: uuid.New(), Name: systemOrgName}
	if err := db.Create(&org).Error; err != nil {
		return uuid.Nil, err
	}
	return org.ID, nil
}

// ownerOrgFor resolves the user's owner membership, creating a personal org + owner membership when they have none yet.
func ownerOrgFor(db *gorm.DB, user models.User) (orgID uuid.UUID, created bool, err error) {
	var memberships int64
	if err := db.Model(&models.Membership{}).Where("user_id = ?", user.ID).Count(&memberships).Error; err != nil {
		return uuid.Nil, false, err
	}
	if memberships == 0 {
		return createPersonalOrg(db, user)
	}
	var owner models.Membership
	err = db.Where("user_id = ? AND role = ?", user.ID, models.RoleOwner).Order("created_at, id").First(&owner).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Staff-only: stamping runs per user_id, so no other user's pass covers
		// these rows. If unstamped legacy rows exist, mint them a personal org so
		// they land in a tenant instead of staying org-less forever; without rows
		// there is nothing to claim (an invite join deliberately has no personal org).
		if !hasUnstampedRows(db, user.ID) {
			return uuid.Nil, false, nil
		}
		return createPersonalOrg(db, user)
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return owner.OrgID, true, nil
}

// createPersonalOrg provisions the user's personal owner org + membership in one transaction (both rows or neither).
func createPersonalOrg(db *gorm.DB, user models.User) (orgID uuid.UUID, created bool, err error) {
	org := models.Organization{ID: uuid.New(), Name: defaultOrgName(user)}
	membership := models.Membership{ID: uuid.New(), OrgID: org.ID, UserID: user.ID, Role: models.RoleOwner}
	// Both rows or neither: a lone org would strand the user with an owner membership pointing at nothing.
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&org).Error; err != nil {
			return err
		}
		return tx.Create(&membership).Error
	})
	if err != nil {
		return uuid.Nil, false, err
	}
	return org.ID, true, nil
}

// hasUnstampedRows reports whether the user holds any legacy row that still needs a
// tenant (org_id NULL/Nil) and that no other user's stamp would cover. The audit trail
// is excluded: every register writes an org-less "auth.register" entry and audit stays
// cross-org anyway, so counting it would mint a personal org for every invited staff
// member. Settings only count when org_id is the primary key: on a v15-upgraded table
// the user_id key is the intended per-user tenant (kept by rebuildSettingsPK).
func hasUnstampedRows(db *gorm.DB, userID uuid.UUID) bool {
	for _, target := range []any{&models.Invoice{}, &models.Client{}, &models.Item{}, &models.Expense{}, &models.Payment{}} {
		if hasUnstampedRowsFor(db, target, "user_id = ? AND (org_id IS NULL OR org_id = ?)", userID, uuid.Nil) {
			return true
		}
	}
	if hasUnstampedRowsFor(db, &models.GatewayProject{}, "owner_user_id = ? AND (org_id IS NULL OR org_id = ?)", userID, uuid.Nil) {
		return true
	}
	return hasUnstampedSettings(db, userID)
}

// hasUnstampedRowsFor counts matching rows; a query error is treated as "no rows" so a probe failure never mints an org by accident.
func hasUnstampedRowsFor(db *gorm.DB, model any, where string, args ...any) bool {
	var count int64
	return db.Model(model).Where(where, args...).Count(&count).Error == nil && count > 0
}

// hasUnstampedSettings reports unstamped settings rows only when org_id is already the settings key; a user_id-keyed table is left to rebuildSettingsPK.
func hasUnstampedSettings(db *gorm.DB, userID uuid.UUID) bool {
	if !db.Migrator().HasColumn(&models.Settings{}, "org_id") || !settingsOrgIDIsPK(db) {
		return false
	}
	var count int64
	return db.Model(&models.Settings{}).
		Where("user_id = ? AND (org_id IS NULL OR org_id = ?)", userID, uuid.Nil).Count(&count).Error == nil && count > 0
}

// settingsOrgIDIsPK reports whether org_id is the settings primary key (fresh schema) rather than a plain column (v15 upgrade).
func settingsOrgIDIsPK(db *gorm.DB) bool {
	cols, err := db.Migrator().ColumnTypes("settings")
	if err != nil {
		return false
	}
	for _, col := range cols {
		if col.Name() != "org_id" {
			continue
		}
		pk, ok := col.PrimaryKey()
		return ok && pk
	}
	return false
}

// defaultOrgName uses the user's name, falling back to the email local part, then a generic label.
func defaultOrgName(user models.User) string {
	if name := strings.TrimSpace(user.Name); name != "" {
		return name
	}
	if local, _, found := strings.Cut(strings.TrimSpace(user.Email), "@"); found && local != "" {
		return local
	}
	return "Organization"
}

// stampOrgColumns batch-fills org_id for the user's rows, skipping rows that already carry one; user_id is never rewritten.
func stampOrgColumns(db *gorm.DB, userID, orgID uuid.UUID) error {
	targets := []any{
		&models.Invoice{}, &models.Client{}, &models.Item{}, &models.Expense{}, &models.Payment{}, &models.AuditLog{},
	}
	for _, target := range targets {
		res := db.Model(target).
			Where("user_id = ? AND (org_id IS NULL OR org_id = ?)", userID, uuid.Nil).
			UpdateColumn("org_id", orgID)
		if res.Error != nil {
			return res.Error
		}
	}
	// Gateway projects backfill from owner_user_id (D8); projects without an owner keep a NULL org_id.
	res := db.Model(&models.GatewayProject{}).
		Where("owner_user_id = ? AND (org_id IS NULL OR org_id = ?)", userID, uuid.Nil).
		UpdateColumn("org_id", orgID)
	if res.Error != nil {
		return res.Error
	}
	return stampSettingsOrg(db, userID, orgID)
}

// stampSettingsOrg writes settings.org_id when the column exists; rows keep their own key otherwise.
func stampSettingsOrg(db *gorm.DB, userID, orgID uuid.UUID) error {
	if !db.Migrator().HasColumn(&models.Settings{}, "org_id") {
		return nil
	}
	return db.Model(&models.Settings{}).
		Where("user_id = ? AND (org_id IS NULL OR org_id = ?)", userID, uuid.Nil).
		UpdateColumn("org_id", orgID).Error
}

// rebuildSettingsPK is idempotent: a settings table whose org_id is already the primary key (fresh schema) or lacks the column is left alone.
// GORM cannot add a PK to an existing table, so the v16 backfill rebuilds settings once.
func rebuildSettingsPK(db *gorm.DB) error {
	cols, err := db.Migrator().ColumnTypes("settings")
	if err != nil {
		return err
	}
	orgIDSeen, orgIDIsPK := false, false
	for _, col := range cols {
		if col.Name() != "org_id" {
			continue
		}
		orgIDSeen = true
		pk, ok := col.PrimaryKey()
		orgIDIsPK = ok && pk // driver that cannot report PK: rebuild from the model definition
	}
	if !orgIDSeen || orgIDIsPK {
		return nil
	}
	var rows []models.Settings
	if err := db.Find(&rows).Error; err != nil {
		return err
	}
	for i := range rows {
		if rows[i].OrgID == uuid.Nil {
			rows[i].OrgID = rows[i].UserID // unstamped legacy row: keep the old user_id key as its org
		}
	}
	if err := db.Migrator().DropTable(&models.Settings{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&models.Settings{}); err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	return db.CreateInBatches(&rows, 100).Error
}
