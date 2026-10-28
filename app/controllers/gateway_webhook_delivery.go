package controllers

import (
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/logger"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/database"
	"github.com/tertua/tupay/platform/gateway"
	"github.com/tertua/tupay/platform/relay"
)

// queueDownstreamDelivery signs the relay payload and stores it for the outbox
// worker. The project secret is sealed at rest, so it is opened here — a
// missing or mismatched key is a hard error (500, so the provider retries),
// never a silently unsigned or forged event.
func queueDownstreamDelivery(c fiber.Ctx, db *database.Queries, txn models.GatewayTransaction, project models.GatewayProject, notif *gateway.NotificationResult, gatewayName, eventID string) error {
	secret, err := relay.ProjectSecret(project.WebhookSecret)
	if err != nil {
		logger.L().Error("project webhook secret unreadable", "project", project.Slug, "err", err)
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to queue delivery", nil)
	}
	payload, _ := json.Marshal(relay.Payload{
		EventID:         "evt_" + eventID,
		OrderID:         txn.OrderID,
		ProjectSlug:     txn.ProjectSlug,
		Gateway:         gatewayName,
		ExternalOrderID: txn.ExternalOrderID,
		Status:          notif.Status,
		GrossAmountIDR:  notif.GrossMinor,
		AmountDecimal:   notif.GrossDecimal,
		Currency:        notif.Currency,
		TransactionID:   notif.TransactionID,
		PaymentType:     notif.PaymentType,
		PaymentMethod:   txn.PaymentMethod,
		PaidAt:          time.Now().UTC().Format(time.RFC3339),
	})
	delivery := &models.WebhookDelivery{
		ID:          uuid.New(),
		OrderID:     txn.OrderID,
		ProjectSlug: txn.ProjectSlug,
		Gateway:     gatewayName,
		TargetURL:   project.WebhookURL,
		Payload:     string(payload),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Status:      "pending",
	}
	delivery.Signature = relay.SignPayload(payload, secret)
	if err := db.CreateDelivery(delivery); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to queue delivery", nil)
	}
	// The background worker forwards and retries; the provider gets a fast
	// acknowledgement and downstream state is tracked in webhook_deliveries.
	return utils.OK(c, fiber.StatusOK, fiber.Map{"success": true, "queued": true})
}
