package controllers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/pkg/utils"
)

// ListSubscriptions returns one page of subscriptions owned by the current org.
// @Description Get subscriptions of current org.
// @Summary list subscriptions
// @Tags Subscriptions
// @Param page query int false "Page number (default 1)"
// @Param per_page query int false "Items per page (default 20, max 100)"
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /subscriptions [get]
func ListSubscriptions(c fiber.Ctx) error {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	paging := utils.ParsePagination(c)
	subs, err := db.ListSubscriptions(orgID, paging.Limit(), paging.Offset())
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load subscriptions", nil)
	}
	total, err := db.CountSubscriptions(orgID)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to count subscriptions", nil)
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"subscriptions": subs, "meta": paging.Meta(total)})
}

// GetSubscription returns one subscription with its items.
// @Description Get a subscription by ID.
// @Summary get subscription
// @Tags Subscriptions
// @Produce json
// @Param id path string true "Subscription ID"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /subscriptions/{id} [get]
func GetSubscription(c fiber.Ctx) error {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid subscription id", nil)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	sub, err := db.GetSubscription(orgID, id)
	if err != nil {
		return utils.NotFoundOrFailed(c, err, "subscription")
	}
	items, err := db.GetSubscriptionItems(id)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load subscription items", nil)
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"subscription": sub, "items": items})
}
