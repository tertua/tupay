package outbox

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/utils"
)

// TestNextSubscriptionRun covers the pure cadence arithmetic, including the
// month-end clamp (Jan 31 -> Feb 28/29, Aug 31 -> Sep 30) and weekly steps.
func TestNextSubscriptionRun(t *testing.T) {
	day := func(y int, m time.Month, d int) time.Time {
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}
	tests := []struct {
		name    string
		from    time.Time
		cadence string
		anchor  int
		want    time.Time
	}{
		{"weekly steps 7 days", day(2026, time.March, 1), models.SubscriptionCadenceWeekly, 1, day(2026, time.March, 8)},
		{"weekly across month boundary", day(2026, time.January, 28), models.SubscriptionCadenceWeekly, 28, day(2026, time.February, 4)},
		{"monthly same day", day(2026, time.January, 15), models.SubscriptionCadenceMonthly, 15, day(2026, time.February, 15)},
		{"jan 31 clamps to feb 28 (non-leap)", day(2026, time.January, 31), models.SubscriptionCadenceMonthly, 31, day(2026, time.February, 28)},
		{"jan 31 clamps to feb 29 (leap)", day(2024, time.January, 31), models.SubscriptionCadenceMonthly, 31, day(2024, time.February, 29)},
		{"aug 31 clamps to sep 30", day(2026, time.August, 31), models.SubscriptionCadenceMonthly, 31, day(2026, time.September, 30)},
		{"monthly year rollover", day(2026, time.December, 15), models.SubscriptionCadenceMonthly, 15, day(2027, time.January, 15)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, nextSubscriptionRun(tt.from, tt.cadence, tt.anchor))
		})
	}
}

// TestSubscriptionRunDue covers the lead-days window, pause gate and nil cursor.
func TestSubscriptionRunDue(t *testing.T) {
	today := utils.DateOnly(time.Now())
	target := func(offset int) *time.Time {
		d := today.AddDate(0, 0, offset)
		return &d
	}
	tests := []struct {
		name     string
		next     *time.Time
		lead     int
		paused   bool
		wantDue  bool
		wantDate *time.Time
	}{
		{"due today", target(0), 0, false, true, target(0)},
		{"inside lead window", target(5), 5, false, true, target(5)},
		{"one day before lead window", target(6), 5, false, false, nil},
		{"one day past target", target(-1), 0, false, false, nil},
		{"paused never due", target(0), 0, true, false, nil},
		{"nil cursor", nil, 0, false, false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := models.SubscriptionScanRow{NextRunDate: tt.next, LeadDays: tt.lead, Paused: tt.paused}
			got, due := subscriptionRunDue(row, today)
			assert.Equal(t, tt.wantDue, due)
			if tt.wantDate != nil {
				assert.Equal(t, *tt.wantDate, got)
			}
		})
	}
}

// seedSubscriptionOrg creates an org with an owner, a client, and one active
// monthly subscription due today, returning the org id and subscription id.
func seedSubscriptionOrg(t *testing.T, tweak func(*models.Subscription)) (uuid.UUID, uuid.UUID) {
	t.Helper()
	owner := uuid.New()
	orgID := uuid.New()
	now := time.Now()
	require.NoError(t, rawDB(t).Create(&models.User{ID: owner, Name: "Owner", Email: "owner-" + owner.String()[:8] + "@x.com",
		PasswordHash: "hash", UserStatus: 1, UserRole: "user", CreatedAt: now, UpdatedAt: now}).Error)
	require.NoError(t, rawDB(t).Create(&models.Organization{ID: orgID, Name: "Org", CreatedAt: now}).Error)
	require.NoError(t, rawDB(t).Create(&models.Membership{ID: uuid.New(), OrgID: orgID, UserID: owner, Role: models.RoleOwner, CreatedAt: now}).Error)
	clientID := uuid.New()
	require.NoError(t, rawDB(t).Create(&models.Client{ID: clientID, CreatedAt: now, UserID: owner, OrgID: orgID, Name: "Retainer Co"}).Error)

	today := utils.DateOnly(now)
	sub := models.Subscription{
		ID: uuid.New(), CreatedAt: now, UserID: owner, OrgID: orgID, ClientID: &clientID,
		Name: "Retainer", Status: models.SubscriptionStatusActive, Cadence: models.SubscriptionCadenceMonthly,
		NextRunDate: &today, Currency: "IDR", InvoiceStatus: models.InvoiceStatusSent, DueDays: 30,
	}
	if tweak != nil {
		tweak(&sub)
	}
	require.NoError(t, testDB(t).CreateSubscription(&sub, []models.SubscriptionItem{
		{ID: uuid.New(), Description: "Service", Quantity: 1, Rate: models.MoneyFromMinor(100000), Position: 0},
	}))

	t.Cleanup(func() {
		rawDB(t).Where("org_id = ?", orgID).Delete(&models.SubscriptionRun{})
		rawDB(t).Where("org_id = ?", orgID).Delete(&models.Invoice{})
		rawDB(t).Where("subscription_id = ?", sub.ID).Delete(&models.SubscriptionItem{})
		rawDB(t).Where("org_id = ?", orgID).Delete(&models.Subscription{})
		rawDB(t).Where("org_id = ?", orgID).Delete(&models.Client{})
	})
	return orgID, sub.ID
}

// subscriptionInvoiceCount counts generated invoices for an org.
func subscriptionInvoiceCount(t *testing.T, orgID uuid.UUID) int64 {
	t.Helper()
	var n int64
	require.NoError(t, rawDB(t).Model(&models.Invoice{}).Where("org_id = ?", orgID).Count(&n).Error)
	return n
}

// subscriptionRunCount counts claim rows for an org.
func subscriptionRunCount(t *testing.T, orgID uuid.UUID) int64 {
	t.Helper()
	var n int64
	require.NoError(t, rawDB(t).Model(&models.SubscriptionRun{}).Where("org_id = ?", orgID).Count(&n).Error)
	return n
}

// TestSubscriptionSweepIdempotent proves two ticks generate exactly one invoice and
// one claim row, and that the run row links the invoice.
func TestSubscriptionSweepIdempotent(t *testing.T) {
	orgID, _ := seedSubscriptionOrg(t, nil)

	New().ProcessOnce(context.Background())
	New().ProcessOnce(context.Background())

	assert.Equal(t, int64(1), subscriptionInvoiceCount(t, orgID), "one generated invoice across two ticks")
	assert.Equal(t, int64(1), subscriptionRunCount(t, orgID), "one claim row across two ticks")

	var run models.SubscriptionRun
	require.NoError(t, rawDB(t).Where("org_id = ?", orgID).First(&run).Error)
	assert.NotNil(t, run.InvoiceID, "claim row links the generated invoice")
}

// TestSubscriptionSweepSkipsPaused verifies a paused subscription never generates.
func TestSubscriptionSweepSkipsPaused(t *testing.T) {
	orgID, _ := seedSubscriptionOrg(t, func(sub *models.Subscription) {
		sub.Status = models.SubscriptionStatusPaused
	})

	New().ProcessOnce(context.Background())

	assert.Equal(t, int64(0), subscriptionInvoiceCount(t, orgID))
	assert.Equal(t, int64(0), subscriptionRunCount(t, orgID))
}

// TestSubscriptionSweepFutureNotDue verifies a subscription whose window has not opened
// generates nothing.
func TestSubscriptionSweepFutureNotDue(t *testing.T) {
	orgID, _ := seedSubscriptionOrg(t, func(sub *models.Subscription) {
		future := utils.DateOnly(time.Now()).AddDate(0, 0, 30)
		sub.NextRunDate = &future
	})

	New().ProcessOnce(context.Background())

	assert.Equal(t, int64(0), subscriptionInvoiceCount(t, orgID))
	assert.Equal(t, int64(0), subscriptionRunCount(t, orgID))
}

// TestSubscriptionSweepLeadWindow proves a subscription inside its lead window
// generates early, keyed on the occurrence (target) date.
func TestSubscriptionSweepLeadWindow(t *testing.T) {
	orgID, _ := seedSubscriptionOrg(t, func(sub *models.Subscription) {
		target := utils.DateOnly(time.Now()).AddDate(0, 0, 3)
		sub.NextRunDate = &target
		sub.LeadDays = 5
	})

	New().ProcessOnce(context.Background())

	assert.Equal(t, int64(1), subscriptionInvoiceCount(t, orgID))
	var run models.SubscriptionRun
	require.NoError(t, rawDB(t).Where("org_id = ?", orgID).First(&run).Error)
	assert.Equal(t, utils.DateOnly(time.Now()).AddDate(0, 0, 3), run.RunDate, "claims the target occurrence, not today")
}
