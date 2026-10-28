package controllers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/configs"
	"github.com/tertua/tupay/pkg/constants"
	"github.com/tertua/tupay/pkg/logger"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/database"
	"github.com/tertua/tupay/platform/relay"
)

// ListDeliveries returns one page of recent relay deliveries for admins.
// @Description List relay deliveries.
// @Summary list relay deliveries
// @Tags Admin
// @Produce json
// @Param page query int false "Page number (default 1)"
// @Param per_page query int false "Items per page (default 20, max 100)"
// @Success 200 {object} map[string]interface{}
// @Router /admin/gateway/deliveries [get]
func ListDeliveries(c fiber.Ctx) error {
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	paging := utils.ParsePagination(c)
	rows, err := db.ListRecentDeliveries(paging.Limit(), paging.Offset())
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load deliveries", nil)
	}
	total, err := db.CountDeliveries()
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to count deliveries", nil)
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"deliveries": deliveryResponses(rows), "meta": paging.Meta(total)})
}

// ListMyDeliveries returns deliveries for one order owned by the calling project.
// @Description List own relay deliveries.
// @Summary list own deliveries
// @Tags Gateway
// @Produce json
// @Param order_id query string true "Global order ID"
// @Success 200 {object} map[string]interface{}
// @Router /gateway/deliveries [get]
func ListMyDeliveries(c fiber.Ctx) error {
	project, err := utils.CurrentServiceProject(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized", nil)
	}
	orderID := strings.TrimSpace(c.Query("order_id"))
	if orderID == "" {
		return utils.Fail(c, fiber.StatusBadRequest, "order_id is required", nil)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	txn, err := db.GetTransaction(orderID)
	if err != nil || txn.ProjectSlug != project.Slug {
		return utils.Fail(c, fiber.StatusNotFound, "transaction not found", nil)
	}
	rows, err := db.ListDeliveriesByOrder(orderID)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load deliveries", nil)
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"deliveries": deliveryResponses(rows)})
}

// RetryDelivery re-attempts a failed relay delivery.
// Service keys may retry only their own project; admins may retry any.
// @Description Retry a relay delivery.
// @Summary retry relay delivery
// @Tags Admin
// @Produce json
// @Param id path string true "Delivery ID"
// @Success 200 {object} map[string]interface{}
// @Router /admin/gateway/deliveries/{id}/retry [post]
func RetryDelivery(c fiber.Ctx) error {
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid delivery id", nil)
	}
	delivery, err := db.GetDelivery(id)
	if err != nil {
		return utils.NotFoundOrFailed(c, err, "delivery")
	}
	project, err := db.GetProjectBySlug(delivery.ProjectSlug)
	if err != nil {
		return utils.Fail(c, fiber.StatusNotFound, "project not found", nil)
	}
	payload := []byte(delivery.Payload)
	secret, err := relay.OpenSecret(project.WebhookSecret)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "project secret unavailable", nil)
	}
	signature := relay.SignPayload(payload, secret)
	result, ferr := relay.Forward(c.Context(), delivery.TargetURL, project.Slug, "evt_retry_"+delivery.ID.String(), payload, secret)
	delivery.Attempt++
	delivery.Signature = signature
	delivery.UpdatedAt = time.Now()
	retryAt := time.Now().Add(configs.Get().Gateway.WebhookRetry())
	if ferr != nil {
		delivery.Status = "failed"
		delivery.RespBody = constants.TruncateLog(ferr.Error())
		delivery.NextRetryAt = &retryAt
		saveDeliveryState(db, &delivery)
		return utils.Fail(c, fiber.StatusBadGateway, "retry failed", nil)
	}
	delivery.RespCode = result.StatusCode
	delivery.RespBody = result.Body
	if result.StatusCode >= 200 && result.StatusCode < 300 {
		delivery.Status = "delivered"
		delivery.NextRetryAt = nil
	} else {
		delivery.Status = "failed"
		delivery.NextRetryAt = &retryAt
	}
	saveDeliveryState(db, &delivery)
	return utils.OK(c, fiber.StatusOK, fiber.Map{"delivery": deliveryResponse(delivery)})
}

// saveDeliveryState persists the delivery row, logging (never propagating) errors.
func saveDeliveryState(db *database.Queries, d *models.WebhookDelivery) {
	if err := db.SaveDelivery(d); err != nil {
		logger.L().Warn("delivery state store failed", "delivery_id", d.ID.String(), "err", err)
	}
}

func deliveryResponse(d models.WebhookDelivery) deliveryResponseRow {
	return deliveryResponseRow{
		ID:          d.ID,
		OrderID:     d.OrderID,
		ProjectSlug: d.ProjectSlug,
		Gateway:     d.Gateway,
		TargetURL:   d.TargetURL,
		Attempt:     d.Attempt,
		Status:      d.Status,
		RespCode:    d.RespCode,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

type deliveryResponseRow struct {
	ID          uuid.UUID `json:"id"`
	OrderID     string    `json:"order_id"`
	ProjectSlug string    `json:"project_slug"`
	Gateway     string    `json:"gateway"`
	TargetURL   string    `json:"target_url"`
	Attempt     int       `json:"attempt"`
	Status      string    `json:"status"`
	RespCode    int       `json:"resp_code"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func deliveryResponses(rows []models.WebhookDelivery) []deliveryResponseRow {
	out := make([]deliveryResponseRow, 0, len(rows))
	for _, d := range rows {
		out = append(out, deliveryResponse(d))
	}
	return out
}
