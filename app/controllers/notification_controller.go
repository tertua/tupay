package controllers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/relay"
)

// sanitizeEvents keeps only known subscribable events; empty means all.
func sanitizeEvents(raw string) string {
	kept := []string{}
	seen := map[string]bool{}
	for _, e := range models.ParseNotifEvents(raw) {
		if !models.NotifEventAllowed(e) || seen[e] {
			continue
		}
		seen[e] = true
		kept = append(kept, e)
	}
	return strings.Join(kept, ",")
}

// CreateEndpoint registers a webhook target for the current user.
// D11: inbox, deliveries & endpoints stay user-scoped; org fan-out writes one row per member (via enqueueOrgNotification).
// @Description Register a notification webhook endpoint.
// @Summary create notification endpoint
// @Tags Notifications
// @Accept json
// @Produce json
// @Param request body models.CreateEndpointInput true "Endpoint payload"
// @Success 201 {object} map[string]interface{}
// @Security SessionCookie
// @Router /notifications/endpoints [post]
func CreateEndpoint(c fiber.Ctx) error {
	userID, err := utils.CurrentUserID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	input := &models.CreateEndpointInput{}
	if err := c.Bind().Body(input); err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid request body", nil)
	}
	if err := utils.NewValidator().Struct(input); err != nil {
		return utils.ValidationFailed(c, err)
	}
	target := strings.TrimSpace(input.TargetURL)
	if !validateEndpointURL(target) {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid target_url, must be http(s) and reachable", nil)
	}
	secret, err := relay.GenerateSecret()
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to generate secret", nil)
	}
	// Only the sealed form is persisted; the raw secret goes back to the caller
	// exactly once (here), so a database dump never yields a usable secret.
	sealed, err := relay.SealSecret(secret)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to protect endpoint secret", nil)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	e := &models.NotificationEndpoint{
		UserID:    userID,
		TargetURL: target,
		Secret:    sealed,
		Events:    sanitizeEvents(input.Events),
		IsActive:  true,
	}
	if err := db.CreateEndpoint(e); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to create endpoint", nil)
	}
	recordAudit(c, db, userID, "notification.endpoint.create", "notification_endpoint", e.ID.String(), "")
	out := endpointResponse(*e)
	out.Secret = secret
	return utils.OK(c, fiber.StatusCreated, fiber.Map{"endpoint": out})
}

// ListEndpoints returns all webhook targets of the current user.
// @Description List notification webhook endpoints.
// @Summary list notification endpoints
// @Tags Notifications
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /notifications/endpoints [get]
func ListEndpoints(c fiber.Ctx) error {
	userID, err := utils.CurrentUserID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	rows, err := db.ListEndpoints(userID)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load endpoints", nil)
	}
	out := make([]endpointResponseRow, 0, len(rows))
	for _, e := range rows {
		out = append(out, endpointResponse(e))
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"endpoints": out})
}

// UpdateEndpoint edits endpoint metadata.
// @Description Update a notification webhook endpoint.
// @Summary update notification endpoint
// @Tags Notifications
// @Accept json
// @Produce json
// @Param id path string true "Endpoint ID"
// @Param request body models.UpdateEndpointInput true "Endpoint payload"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /notifications/endpoints/{id} [patch]
func UpdateEndpoint(c fiber.Ctx) error {
	userID, err := utils.CurrentUserID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid endpoint id", nil)
	}
	input := &models.UpdateEndpointInput{}
	if err := c.Bind().Body(input); err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid request body", nil)
	}
	if err := utils.NewValidator().Struct(input); err != nil {
		return utils.ValidationFailed(c, err)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	e, err := db.GetEndpoint(userID, id)
	if err != nil {
		return utils.NotFoundOrFailed(c, err, "endpoint")
	}
	if strings.TrimSpace(input.TargetURL) != "" {
		target := strings.TrimSpace(input.TargetURL)
		if !validateEndpointURL(target) {
			return utils.Fail(c, fiber.StatusBadRequest, "invalid target_url, must be http(s) and reachable", nil)
		}
		e.TargetURL = target
	}
	// Empty string clears the filter (subscribe to all); unknown tokens are dropped by sanitizeEvents, never stored.
	if input.Events != nil {
		e.Events = sanitizeEvents(*input.Events)
	}
	if input.IsActive != nil {
		e.IsActive = *input.IsActive
	}
	// A legacy row may still hold a plaintext secret: upgrade it while the row
	// is here, so setting SECRET_ENCRYPTION_KEY also seals existing endpoints.
	sealed, err := relay.SealStored(e.Secret)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to protect endpoint secret", nil)
	}
	e.Secret = sealed
	e.UpdatedAt = time.Now()
	if err := db.SaveEndpoint(&e); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to update endpoint", nil)
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"endpoint": endpointResponse(e)})
}

// DeleteEndpoint removes an endpoint of the current user.
// @Description Delete a notification webhook endpoint.
// @Summary delete notification endpoint
// @Tags Notifications
// @Produce json
// @Param id path string true "Endpoint ID"
// @Success 204 {string} status "ok"
// @Security SessionCookie
// @Router /notifications/endpoints/{id} [delete]
func DeleteEndpoint(c fiber.Ctx) error {
	userID, err := utils.CurrentUserID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid endpoint id", nil)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	if _, err := db.GetEndpoint(userID, id); err != nil {
		return utils.NotFoundOrFailed(c, err, "endpoint")
	}
	if err := db.DeleteEndpoint(userID, id); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to delete endpoint", nil)
	}
	recordAudit(c, db, userID, "notification.endpoint.delete", "notification_endpoint", id.String(), "")
	return c.SendStatus(fiber.StatusNoContent)
}

// RotateEndpointSecret issues a new signing secret for an endpoint.
// @Description Rotate a notification endpoint secret.
// @Summary rotate endpoint secret
// @Tags Notifications
// @Produce json
// @Param id path string true "Endpoint ID"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /notifications/endpoints/{id}/rotate-secret [post]
func RotateEndpointSecret(c fiber.Ctx) error {
	userID, err := utils.CurrentUserID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid endpoint id", nil)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	e, err := db.GetEndpoint(userID, id)
	if err != nil {
		return utils.NotFoundOrFailed(c, err, "endpoint")
	}
	secret, err := relay.GenerateSecret()
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to generate secret", nil)
	}
	sealed, err := relay.SealSecret(secret)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to protect endpoint secret", nil)
	}
	e.Secret = sealed
	e.UpdatedAt = time.Now()
	if err := db.SaveEndpoint(&e); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to rotate secret", nil)
	}
	recordAudit(c, db, userID, "notification.endpoint.rotate", "notification_endpoint", e.ID.String(), "")
	out := endpointResponse(e)
	out.Secret = secret
	return utils.OK(c, fiber.StatusOK, fiber.Map{"endpoint": out})
}

// TestEndpoint enqueues a notification.test event to one endpoint.
// @Description Send a test notification.
// @Summary test notification endpoint
// @Tags Notifications
// @Produce json
// @Param id path string true "Endpoint ID"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /notifications/endpoints/{id}/test [post]
func TestEndpoint(c fiber.Ctx) error {
	userID, err := utils.CurrentUserID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid endpoint id", nil)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	e, err := db.GetEndpoint(userID, id)
	if err != nil {
		return utils.NotFoundOrFailed(c, err, "endpoint")
	}
	eventHex, _ := randHex(8)
	enqueueNotificationDelivery(db, userID, e, models.NotifEventTest, "evt_"+eventHex,
		fiber.Map{"endpoint_id": e.ID.String(), "target_url": e.TargetURL})
	return utils.OK(c, fiber.StatusOK, fiber.Map{"queued": true})
}

// ListNotificationDeliveries returns one page of deliveries for the user.
// @Description List notification deliveries.
// @Summary list notification deliveries
// @Tags Notifications
// @Produce json
// @Param event_type query string false "Filter by event type"
// @Param status query string false "Filter by status"
// @Param page query int false "Page number (default 1)"
// @Param per_page query int false "Items per page (default 20, max 100)"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /notifications/deliveries [get]
func ListNotificationDeliveries(c fiber.Ctx) error {
	userID, err := utils.CurrentUserID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	eventType := strings.TrimSpace(c.Query("event_type"))
	status := strings.TrimSpace(c.Query("status"))
	paging := utils.ParsePagination(c)
	rows, err := db.ListDeliveriesByUser(userID, eventType, status, paging.Limit(), paging.Offset())
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load deliveries", nil)
	}
	total, err := db.CountDeliveriesByUser(userID, eventType, status)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to count deliveries", nil)
	}
	out := make([]notificationDeliveryRow, 0, len(rows))
	for _, d := range rows {
		out = append(out, notificationDeliveryResponse(d))
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"deliveries": out, "meta": paging.Meta(total)})
}

// RetryNotificationDelivery re-attempts a failed delivery of the user.
// @Description Retry a notification delivery.
// @Summary retry notification delivery
// @Tags Notifications
// @Produce json
// @Param id path string true "Delivery ID"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /notifications/deliveries/{id}/retry [post]
func RetryNotificationDelivery(c fiber.Ctx) error {
	userID, err := utils.CurrentUserID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid delivery id", nil)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	d, err := db.GetDeliveryByUser(userID, id)
	if err != nil {
		return utils.NotFoundOrFailed(c, err, "delivery")
	}
	if _, err := db.GetEndpoint(userID, d.EndpointID); err != nil {
		return utils.Fail(c, fiber.StatusNotFound, "endpoint not found", nil)
	}
	if err := db.RequeueNotification(d.ID, time.Now()); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to requeue delivery", nil)
	}
	updated, err := db.GetDeliveryByUser(userID, id)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load delivery", nil)
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"delivery": notificationDeliveryResponse(updated)})
}
