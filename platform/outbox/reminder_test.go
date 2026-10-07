package outbox

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/app/queries"
	"github.com/tertua/tupay/pkg/utils"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// rawDB exposes the shared gorm handle behind the domain query methods
// (database.Queries embeds many *gorm.DB, so db.Create/db.Model are ambiguous).
func rawDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testDB(t).InvoiceQueries.DB
}

// TestReminderLeg covers the per-org before/after window arithmetic with no DB.
func TestReminderLeg(t *testing.T) {
	today := utils.DateOnly(time.Now())
	day := func(offset int) *time.Time {
		d := today.AddDate(0, 0, offset)
		return &d
	}
	tests := []struct {
		name   string
		due    *time.Time
		before int
		after  int
		want   string
	}{
		{"before window opens", day(7), 7, 0, models.InvoiceReminderKindBefore},
		{"due today is not before", day(0), 7, 0, ""},
		{"inside no window", day(3), 2, 0, ""},
		{"after window opens", day(-3), 0, 3, models.InvoiceReminderKindAfter},
		{"not yet after", day(-2), 0, 3, ""},
		{"both legs off", day(-30), 0, 0, ""},
		{"before off after off", day(1), 0, 0, ""},
		{"nil due date", nil, 7, 3, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := models.ReminderScanRow{DueDate: tt.due, BeforeDays: tt.before, AfterDays: tt.after, Enabled: true}
			assert.Equal(t, tt.want, reminderLeg(row, today))
		})
	}
}

// TestReminderLegDisabled verifies the org switch suppresses both legs.
func TestReminderLegDisabled(t *testing.T) {
	today := utils.DateOnly(time.Now())
	due := today.AddDate(0, 0, 7)
	row := models.ReminderScanRow{DueDate: &due, BeforeDays: 7, AfterDays: 3, Enabled: false}
	assert.Equal(t, "", reminderLeg(row, today))
}

// seedReminderOrg creates an org with one owner, settings, and returns the org,
// user, and a sent invoice due offset days from today.
func seedReminderOrg(t *testing.T, dueOffset int, tweak func(*models.Settings)) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	owner := uuid.New()
	orgID := uuid.New()
	now := time.Now()
	require.NoError(t, rawDB(t).Create(&models.User{ID: owner, Name: "Owner", Email: "owner-" + owner.String()[:8] + "@x.com",
		PasswordHash: "hash", UserStatus: 1, UserRole: "user", CreatedAt: now, UpdatedAt: now}).Error)
	require.NoError(t, rawDB(t).Create(&models.Organization{ID: orgID, Name: "Org", CreatedAt: now}).Error)
	require.NoError(t, rawDB(t).Create(&models.Membership{ID: uuid.New(), OrgID: orgID, UserID: owner, Role: models.RoleOwner, CreatedAt: now}).Error)

	settings := models.DefaultSettings(owner)
	settings.OrgID = orgID
	require.NoError(t, rawDB(t).Create(settings).Error)
	// The reminder columns carry GORM default tags, so an explicit zero/false
	// must go through a map update (Create drops zero values and the DB default
	// wins). This mirrors the API path (UpdateSettings).
	if tweak != nil {
		tweak(settings)
		require.NoError(t, testDB(t).UpdateSettings(settings))
	}

	due := utils.DateOnly(now).AddDate(0, 0, dueOffset)
	invoice := models.Invoice{
		ID: uuid.New(), CreatedAt: now, UserID: owner, OrgID: orgID,
		InvoiceNumber: "INV-REM-" + uuid.NewString()[:6], Status: models.InvoiceStatusSent,
		DueDate: &due, Currency: "IDR", Total: models.MoneyFromMinor(250000),
	}
	require.NoError(t, rawDB(t).Create(&invoice).Error)
	// The outbox package shares one in-memory DB across tests, and the reminder
	// sweep is global: remove this org's rows so later tests (mail retry counts,
	// etc.) never see leftover reminder candidates or queued mails.
	t.Cleanup(func() {
		rawDB(t).Where("org_id = ?", orgID).Delete(&models.Invoice{})
		rawDB(t).Where("org_id = ?", orgID).Delete(&models.InvoiceReminderLog{})
		rawDB(t).Where("org_id = ?", orgID).Delete(&models.Settings{})
		// Reminder mails are addressed to this org's billing/owner email, unique
		// per test: drop them so they never linger in the shared outbox queue.
		rawDB(t).Where("`to` LIKE ?", "owner-"+owner.String()[:8]+"@%").Delete(&models.MailOutbox{})
		rawDB(t).Where("`to` LIKE ?", "billing-%@x.com").Delete(&models.MailOutbox{})
	})
	return orgID, owner, invoice.ID
}

// reminderLogCount counts reminder log rows for one invoice.
func reminderLogCount(t *testing.T, invoiceID uuid.UUID, kind string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, rawDB(t).Model(&models.InvoiceReminderLog{}).
		Where("invoice_id = ? AND kind = ?", invoiceID, kind).Count(&n).Error)
	return n
}

// mailCount counts queued reminder mails addressed to a recipient.
func mailCount(t *testing.T, to string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, rawDB(t).Model(&models.MailOutbox{}).Where("`to` = ?", to).Count(&n).Error)
	return n
}

// TestReminderSweepClaimsLeg verifies a due invoice mints one log row + mail.
func TestReminderSweepClaimsLeg(t *testing.T) {
	_, owner, invoiceID := seedReminderOrg(t, 7, nil)
	ownerUser, err := testDB(t).GetUserByID(owner)
	require.NoError(t, err)

	New().ProcessOnce(context.Background())

	assert.Equal(t, int64(1), reminderLogCount(t, invoiceID, models.InvoiceReminderKindBefore))
	assert.Equal(t, int64(1), mailCount(t, ownerUser.Email))
}

// errRecordingLogger captures GORM statement errors so a test can prove no
// failing statement was executed at all. GORM reports failed SQL through
// Trace(err), so that is the capture point.
type errRecordingLogger struct{ errs []string }

func (l *errRecordingLogger) LogMode(gormlogger.LogLevel) gormlogger.Interface { return l }
func (l *errRecordingLogger) Info(context.Context, string, ...interface{})     {}
func (l *errRecordingLogger) Warn(context.Context, string, ...interface{})     {}
func (l *errRecordingLogger) Error(_ context.Context, msg string, _ ...interface{}) {
	l.errs = append(l.errs, msg)
}
func (l *errRecordingLogger) Trace(_ context.Context, _ time.Time, _ func() (string, int64), err error) {
	if err != nil {
		l.errs = append(l.errs, err.Error())
	}
}

// TestClaimSkipsDoomedInsert proves the pre-check path: once a leg is claimed,
// re-claiming reports (false, nil) without executing — and failing — the
// unique insert. Without the pre-check, a permanently-due after-leg would hit
// the duplicate key on every 10s sweep tick and GORM would log it as an error.
// The insert stays the cross-replica race arbiter (covered by
// TestReminderSweepIdempotent and TestReminderTwoLegsSendSeparately).
func TestClaimSkipsDoomedInsert(t *testing.T) {
	orgID, _, invoiceID := seedReminderOrg(t, 7, nil)
	now := time.Now()

	claimed, err := testDB(t).ClaimInvoiceReminder(orgID, invoiceID, models.InvoiceReminderKindBefore, now)
	require.NoError(t, err)
	require.True(t, claimed)

	rec := &errRecordingLogger{}
	q := queries.InvoiceReminderQueries{DB: rawDB(t).Session(&gorm.Session{Logger: rec})}
	claimed, err = q.ClaimInvoiceReminder(orgID, invoiceID, models.InvoiceReminderKindBefore, now)
	require.NoError(t, err)
	assert.False(t, claimed)
	assert.Empty(t, rec.errs, "re-claim of an already-claimed leg must not run a failing statement")
}

// TestReminderSweepIdempotent verifies a second tick never re-reminds.
func TestReminderSweepIdempotent(t *testing.T) {
	_, _, invoiceID := seedReminderOrg(t, 7, nil)

	New().ProcessOnce(context.Background())
	New().ProcessOnce(context.Background())

	assert.Equal(t, int64(1), reminderLogCount(t, invoiceID, models.InvoiceReminderKindBefore))
}

// TestReminderSweepSkipsUnsent verifies draft/paid invoices are never reminded.
func TestReminderSweepSkipsUnsent(t *testing.T) {
	_, owner, paidID := seedReminderOrg(t, 7, nil)
	require.NoError(t, rawDB(t).Model(&models.Invoice{}).Where("id = ?", paidID).Update("status", models.InvoiceStatusPaid).Error)
	_ = owner

	New().ProcessOnce(context.Background())

	assert.Equal(t, int64(0), reminderLogCount(t, paidID, models.InvoiceReminderKindBefore))
}

// TestReminderSweepDisabledOrg verifies the org switch suppresses reminders.
func TestReminderSweepDisabledOrg(t *testing.T) {
	_, _, invoiceID := seedReminderOrg(t, 7, func(s *models.Settings) { s.ReminderEnabled = false })

	New().ProcessOnce(context.Background())

	assert.Equal(t, int64(0), reminderLogCount(t, invoiceID, models.InvoiceReminderKindBefore))
}

// TestReminderSweepBothLegsOff verifies 0/0 disables the schedule.
func TestReminderSweepBothLegsOff(t *testing.T) {
	_, _, invoiceID := seedReminderOrg(t, 7, func(s *models.Settings) {
		s.ReminderBeforeDays = 0
		s.ReminderAfterDays = 0
	})

	New().ProcessOnce(context.Background())

	assert.Equal(t, int64(0), reminderLogCount(t, invoiceID, models.InvoiceReminderKindBefore))
}

// TestReminderTwoLegsSendSeparately verifies before and after are independent
// legs: an invoice reminded early still gets the overdue reminder later.
func TestReminderTwoLegsSendSeparately(t *testing.T) {
	_, owner, invoiceID := seedReminderOrg(t, 7, nil)
	ownerUser, err := testDB(t).GetUserByID(owner)
	require.NoError(t, err)

	New().ProcessOnce(context.Background())
	// Move the invoice past its after window and sweep again.
	past := utils.DateOnly(time.Now()).AddDate(0, 0, -5)
	require.NoError(t, rawDB(t).Model(&models.Invoice{}).Where("id = ?", invoiceID).Update("due_date", past).Error)
	New().ProcessOnce(context.Background())

	assert.Equal(t, int64(1), reminderLogCount(t, invoiceID, models.InvoiceReminderKindBefore))
	assert.Equal(t, int64(1), reminderLogCount(t, invoiceID, models.InvoiceReminderKindAfter))
	assert.Equal(t, int64(2), mailCount(t, ownerUser.Email))
}

// TestReminderBillingEmailWins verifies the org billing email receives the mail.
func TestReminderBillingEmailWins(t *testing.T) {
	billing := "billing-" + uuid.NewString()[:8] + "@x.com"
	_, _, invoiceID := seedReminderOrg(t, 7, func(s *models.Settings) { s.Email = billing })

	New().ProcessOnce(context.Background())

	assert.Equal(t, int64(1), reminderLogCount(t, invoiceID, models.InvoiceReminderKindBefore))
	assert.Equal(t, int64(1), mailCount(t, billing))
}
