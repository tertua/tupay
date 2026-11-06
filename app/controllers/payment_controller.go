package controllers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/app/queries"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/database"
	"gorm.io/gorm"
)

func newPaymentToken() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

// reopenInvoiceIfUnderpaid flips a paid invoice back to sent when the void
// dropped its covered amount below the total. Best-effort: a failure here must
// not undo the void (the audit row already records it).
func reopenInvoiceIfUnderpaid(db *database.Queries, orgID, invoiceID uuid.UUID) {
	invoice, err := db.GetInvoice(orgID, invoiceID)
	if err != nil || invoice.Status != models.InvoiceStatusPaid {
		return
	}
	remaining, err := db.PaidAmount(invoiceID)
	if err != nil || remaining.GreaterThanOrEqual(invoice.Total) {
		return
	}
	_ = db.UpdateInvoiceStatus(orgID, invoiceID, models.InvoiceStatusSent)
}

// ListPayments returns one page of payments plus global totals.
// @Description Get payments of current user.
// @Summary get payments
// @Tags Payments
// @Param page query int false "Page number (default 1)"
// @Param per_page query int false "Items per page (default 20, max 100)"
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /payments [get]
func ListPayments(c fiber.Ctx) error {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	paging := utils.ParsePagination(c)
	rows, err := db.ListPayments(orgID, paging.Limit(), paging.Offset())
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load payments", nil)
	}
	total, err := db.CountPayments(orgID)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to count payments", nil)
	}
	totals, err := db.GetPaymentTotals(orgID)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load payment totals", nil)
	}
	payments := make([]paymentListResponseRow, 0, len(rows))
	for _, row := range rows {
		payments = append(payments, paymentListResponse(row))
	}
	return utils.OK(c, fiber.StatusOK, paymentListPayload{
		Payments: payments,
		Totals: fiber.Map{
			"total":     totals.Total,
			"thisMonth": totals.ThisMonth,
		},
		Meta: paging.Meta(total),
	})
}

// CreatePayment records a payment against an invoice owned by the current user.
// @Description Record a payment.
// @Summary create payment
// @Tags Payments
// @Accept json
// @Produce json
// @Param request body models.PaymentInput true "Payment payload"
// @Success 201 {object} map[string]interface{}
// @Security SessionCookie
// @Param Idempotency-Key header string false "Replay protection key (uuid per payment intent)"
// @Router /payments [post]
func CreatePayment(c fiber.Ctx) error {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	input := &models.PaymentInput{}
	if err := c.Bind().Body(input); err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid request body", nil)
	}
	if err := utils.NewValidator().Struct(input); err != nil {
		return utils.ValidationFailed(c, err)
	}
	if !input.Amount.GreaterThan(decimal.Zero) {
		return utils.Fail(c, fiber.StatusBadRequest, "amount must be greater than zero", nil)
	}
	invoiceID, err := uuid.Parse(input.InvoiceID)
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid invoice id", nil)
	}
	paidOn, err := utils.ParseDate(input.PaidOn)
	if err != nil || paidOn == nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid paid_on, expected YYYY-MM-DD", nil)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	if _, err := db.GetInvoice(orgID, invoiceID); err != nil {
		return utils.NotFoundOrFailed(c, err, "invoice")
	}
	// Money in flight locks manual payments: a concurrent manual record would settle against a different balance than the Snap intent.
	if db.PendingInvoiceIDs(orgID)[invoiceID] {
		return utils.Fail(c, fiber.StatusUnprocessableEntity, "invoice has a pending payment", nil)
	}
	now := time.Now()
	payment := &models.Payment{
		ID:        uuid.New(),
		CreatedAt: now,
		UserID:    utils.CurrentActorID(c),
		OrgID:     orgID,
		InvoiceID: invoiceID,
		Amount:    input.Amount,
		Method:    input.Method,
		PaidOn:    paidOn,
		TxnID:     input.TxnID,
		Notes:     input.Notes,
	}
	// The balance guard runs inside the insert transaction, so two payments
	// recorded at once cannot together exceed the invoice total.
	if err := db.CreatePaymentWithinBalance(payment); err != nil {
		return paymentCreateFailure(c, err)
	}
	recordAudit(c, db, utils.CurrentActorID(c), "payment.create", "payment", payment.ID.String(),
		`{"invoice_id":"`+invoiceID.String()+`","amount":"`+input.Amount.String()+`"}`)
	invalidateAggregates(c, orgID)
	enqueueOrgNotification(db, orgID, models.NotifEventPaymentCreated, "", paymentNotifData(db, *payment))
	return utils.OK(c, fiber.StatusCreated, fiber.Map{"payment": paymentItemResponse(*payment)})
}

// paymentCreateFailure maps the transactional balance guard onto the envelope.
func paymentCreateFailure(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, queries.ErrPaymentExceedsBalance):
		return utils.Fail(c, fiber.StatusBadRequest, "payment exceeds invoice balance", nil)
	case errors.Is(err, sql.ErrNoRows):
		return utils.Fail(c, fiber.StatusNotFound, "invoice not found", nil)
	default:
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to create payment", nil)
	}
}

// VoidPayment voids a payment owned by the current user instead of deleting
// it. The row stays for audit but is excluded from every balance, list and
// aggregate query, so it disappears from the frontend.
// @Description Void a payment.
// @Summary void payment
// @Tags Payments
// @Accept json
// @Produce json
// @Param id path string true "Payment ID"
// @Param reason query string false "Void reason (required, also accepted as JSON body {reason})"
// @Param request body models.PaymentVoidInput false "Void payload (alternative to query)"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Param Idempotency-Key header string false "Replay protection key (uuid per void intent)"
// @Router /payments/{id} [delete]
func VoidPayment(c fiber.Ctx) error {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid payment id", nil)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	payment, err := db.GetPayment(orgID, id)
	if err != nil {
		return utils.NotFoundOrFailed(c, err, "payment")
	}
	if payment.VoidedAt != nil {
		return utils.Fail(c, fiber.StatusConflict, "payment is already voided", nil)
	}
	// Gateway-settled payments are provider ledger entries: voiding them
	// here would silently diverge from the gateway's record.
	if !payment.CanVoid() {
		return utils.Fail(c, fiber.StatusUnprocessableEntity, "gateway payment cannot be voided", nil)
	}
	// A pending overlay locks money movement: voiding would change the
	// balance the live Snap intent settles against.
	if _, err := db.GetInvoice(orgID, payment.InvoiceID); err == nil {
		if db.PendingInvoiceIDs(orgID)[payment.InvoiceID] {
			return utils.Fail(c, fiber.StatusUnprocessableEntity, "invoice has a pending payment", nil)
		}
	}
	reason, rerr := resolveVoidReason(c)
	if rerr != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid request body", nil)
	}
	if reason == "" {
		return utils.Fail(c, fiber.StatusBadRequest, "void reason is required", nil)
	}
	if err := utils.NewValidator().Var(reason, "lte=500"); err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "void reason is too long", nil)
	}
	if err := db.VoidPayment(orgID, id, reason); err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, gorm.ErrRecordNotFound) {
			return utils.Fail(c, fiber.StatusConflict, "payment is already voided", nil)
		}
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to void payment", nil)
	}
	// Reopen the invoice when a void drops payments below the total again.
	reopenInvoiceIfUnderpaid(db, orgID, payment.InvoiceID)
	meta, _ := json.Marshal(paymentAuditMeta{
		InvoiceID: payment.InvoiceID.String(),
		Amount:    payment.Amount,
		Reason:    reason,
	})
	recordAudit(c, db, utils.CurrentActorID(c), "payment.void", "payment", id.String(), string(meta))
	voidData := paymentNotifData(db, payment)
	voidData.VoidReason = reason
	enqueueOrgNotification(db, orgID, models.NotifEventPaymentVoided, "", voidData)
	invalidateAggregates(c, orgID)
	return utils.OK(c, fiber.StatusOK, fiber.Map{"payment": paymentVoidResponse(payment)})
}
