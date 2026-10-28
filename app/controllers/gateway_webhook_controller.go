package controllers

import (
	"database/sql"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/logger"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/cache"
	"github.com/tertua/tupay/platform/gateway"
)

// HandleGatewayWebhook handles notifications for any registered gateway.
// @Description Handle a payment gateway notification.
// @Summary gateway webhook
// @Tags Webhooks
// @Accept json
// @Produce json
// @Param gateway path string true "Gateway name"
// @Param request body object true "Provider notification"
// @Success 200 {object} map[string]interface{}
// @Router /webhooks/{gateway} [post]
func HandleGatewayWebhook(c fiber.Ctx) error {
	return handleGatewayWebhook(c, strings.ToLower(strings.TrimSpace(c.Params("gateway"))))
}

func handleGatewayWebhook(c fiber.Ctx, gatewayName string) error {
	gw, err := gateway.Get(gatewayName)
	if err != nil {
		return utils.Fail(c, fiber.StatusNotFound, "unknown payment gateway", nil)
	}
	raw := append([]byte(nil), c.Body()...)
	notif, err := gw.ParseAndVerify(raw)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrNotConfigured):
			return utils.Fail(c, fiber.StatusNotImplemented, "payment gateway is not configured", nil)
		case errors.Is(err, gateway.ErrInvalidSignature):
			return utils.Fail(c, fiber.StatusUnauthorized, "invalid signature", nil)
		default:
			return utils.Fail(c, fiber.StatusBadRequest, "invalid notification", nil)
		}
	}
	status := notif.Status

	db, ok := openDB(c)
	if !ok {
		return nil
	}
	eventID, _ := randHex(8)
	event := &models.GatewayEvent{
		ID:        uuid.New(),
		OrderID:   notif.OrderID,
		Source:    gatewayName,
		Verified:  true,
		Payload:   string(raw),
		CreatedAt: time.Now(),
	}
	if err := db.CreateEvent(event); err != nil {
		logger.L().Error("webhook event audit failed", "order_id", notif.OrderID, "gateway", gatewayName, "err", err)
	}

	txn, err := db.GetTransaction(notif.OrderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return utils.Fail(c, fiber.StatusNotFound, "unknown order", nil)
		}
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load transaction", nil)
	}

	statusChanged := txn.Status != status
	// Idempotent: same status + same gateway txn id needs no work unless no delivery ever succeeded (forward may have failed earlier).
	if !statusChanged && txn.ProviderTxnID == notif.TransactionID {
		if deliveries, derr := db.ListDeliveriesByOrder(txn.OrderID); derr == nil {
			if slices.ContainsFunc(deliveries, func(d models.WebhookDelivery) bool {
				return d.Status == "delivered"
			}) {
				return utils.OK(c, fiber.StatusOK, fiber.Map{"success": true})
			}
		}
	}

	txn.Status = status
	txn.ProviderTxnID = notif.TransactionID
	txn.PaymentType = notif.PaymentType
	if txn.PaymentMethod == "" {
		txn.PaymentMethod = notif.PaymentMethod
	}
	txn.RawNotification = string(raw)
	txn.UpdatedAt = time.Now()
	if status == models.GatewayStatusSuccess {
		now := time.Now()
		txn.PaidAt = &now
	}
	// Validate webhook amount and currency before settlement to prevent money loss.
	if status == models.GatewayStatusSuccess && txn.ProjectSlug == "local" && txn.InvoiceID != nil && txn.UserID != nil {
		if verr := checkWebhookAmount(c, txn, notif); verr != nil {
			return verr
		}
		if err := db.SaveTransactionAndSettleInvoice(&txn, gateway.SettleAmount(models.MoneyFromMinor(notif.GrossMinor), notif.Currency, txn.InvoiceCurrency, txn.UsdToIdr), gateway.DisplayName(gatewayName)); err != nil {
			return utils.Fail(c, fiber.StatusInternalServerError, "failed to settle invoice payment", nil)
		}
		orgID, oerr := db.OrgIDForTransaction(&txn)
		if ierr := cache.InvalidateOrg(c.Context(), orgID.String()); oerr != nil || ierr != nil {
			logger.L().Warn("cache invalidation failed after payment settlement", "org_id", orgID.String(), "order_id", notif.OrderID, "err", errors.Join(oerr, ierr))
		}
		eventHex, _ := randHex(8)
		enqueueNotification(db, *txn.UserID, models.NotifEventInvoiceStatusUpdated, "evt_"+eventHex,
			invoiceNotifData(db, orgID, *txn.InvoiceID))
	} else if err := db.SaveTransaction(&txn); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to update transaction", nil)
	}

	// Local orders have no downstream project to notify.
	if txn.ProjectSlug == "local" {
		return utils.OK(c, fiber.StatusOK, fiber.Map{"success": true})
	}

	project, err := db.GetProjectBySlug(txn.ProjectSlug)
	if err != nil {
		return utils.Fail(c, fiber.StatusNotFound, "project not found", nil)
	}
	if !project.IsActive || strings.TrimSpace(project.WebhookURL) == "" {
		return utils.OK(c, fiber.StatusOK, fiber.Map{"success": true})
	}

	return queueDownstreamDelivery(c, db, txn, project, notif, gatewayName, eventID)
}
