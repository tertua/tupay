package queries

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/tertua/tupay/app/models"
)

// ErrPaymentExceedsBalance rejects a manual payment that would push an invoice
// past its total. The check runs inside the inserting transaction, so two
// payments recorded at the same moment cannot together overpay.
var ErrPaymentExceedsBalance = errors.New("payment exceeds invoice balance")

// CreatePaymentWithinBalance stores a payment only while the invoice still has
// room for it: the invoice row is locked (PostgreSQL) and the paid sum is read
// in the same transaction that inserts the row. SQLite serializes writers
// itself, so the second concurrent insert fails instead of overpaying.
func (q *PaymentQueries) CreatePaymentWithinBalance(payment *models.Payment) error {
	return DoRetry(func() error {
		return q.Transaction(func(tx *gorm.DB) error {
			var invoice models.Invoice
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND org_id = ?", payment.InvoiceID, payment.OrgID).
				First(&invoice).Error; err != nil {
				return notFound(err)
			}
			var paid decimal.Decimal
			if err := tx.Model(&models.Payment{}).
				Where("invoice_id = ? AND voided_at IS NULL", invoice.ID).
				Select("COALESCE(SUM(amount), 0)").Scan(&paid).Error; err != nil {
				return err
			}
			settled := paid.Add(payment.Amount)
			if settled.GreaterThan(invoice.Total) {
				return ErrPaymentExceedsBalance
			}
			if err := rideOrg(tx, payment, &payment.OrgID, payment.UserID); err != nil {
				return err
			}
			// Auto-mark paid when the new payment covers the total, so the
			// stored status column stays in sync (display uses effective_status).
			if invoice.Status != models.InvoiceStatusPaid && invoice.Total.GreaterThan(decimal.Zero) &&
				settled.GreaterThanOrEqual(invoice.Total) {
				return tx.Model(&models.Invoice{}).Where("id = ?", invoice.ID).
					Updates(map[string]any{
						"status":     models.InvoiceStatusPaid,
						"updated_at": time.Now(),
					}).Error
			}
			return nil
		})
	})
}
