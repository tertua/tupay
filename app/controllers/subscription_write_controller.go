package controllers

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/utils"
)

// subscriptionBuildError maps a subscription payload that fails to build into a 400.
type subscriptionBuildError struct{ msg string }

func (e subscriptionBuildError) Error() string { return e.msg }

// buildSubscriptionInput parses and validates a create/update payload into a
// subscription header plus item rows. It keeps the write handlers thin and is
// the single place the JSON→model mapping lives.
func buildSubscriptionInput(input *models.SubscriptionInput, orgID, userID uuid.UUID) (*models.Subscription, []models.SubscriptionItem, error) {
	clientID, err := parseSubscriptionClient(input)
	if err != nil {
		return nil, nil, err
	}
	// Reuse the invoice send rule: a sent subscription must name a client.
	if err := validateInvoice(input.InvoiceStatus, clientID); err != nil {
		return nil, nil, subscriptionBuildError{msg: err.Error()}
	}
	nextRun, err := parseSubscriptionStart(input.StartDate)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	sub := &models.Subscription{
		ID:            uuid.New(),
		CreatedAt:     now,
		UpdatedAt:     &now,
		UserID:        userID,
		OrgID:         orgID,
		ClientID:      clientID,
		Name:          input.Name,
		Status:        input.Status,
		Cadence:       input.Cadence,
		NextRunDate:   nextRun,
		LeadDays:      input.LeadDays,
		Currency:      input.Currency,
		TaxRate:       input.TaxRate,
		Discount:      input.Discount,
		Notes:         input.Notes,
		Terms:         input.Terms,
		PaymentMethod: input.PaymentMethod,
		InvoiceStatus: input.InvoiceStatus,
		DueDays:       input.DueDays,
	}
	// Build item rows through the shared money math so a negative rate/discount
	// is rejected with the same error as an invoice.
	items, err := subscriptionItemRows(sub.ID, input.Items)
	if err != nil {
		return nil, nil, err
	}
	return sub, items, nil
}

// parseSubscriptionClient resolves the optional client id and rejects a bad one.
func parseSubscriptionClient(input *models.SubscriptionInput) (*uuid.UUID, error) {
	if input.ClientID == nil || *input.ClientID == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(*input.ClientID)
	if err != nil {
		return nil, subscriptionBuildError{msg: "invalid client_id"}
	}
	return &parsed, nil
}

// parseSubscriptionStart parses the date-only start_date into the first run
// cursor. An empty start date leaves the subscription unscheduled (NextRunDate
// nil).
func parseSubscriptionStart(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := utils.ParseRequiredDate(value)
	if err != nil {
		return nil, subscriptionBuildError{msg: "invalid start_date, expected YYYY-MM-DD"}
	}
	day := utils.DateOnly(parsed)
	return &day, nil
}

// subscriptionItemRows builds subscription line rows, reusing the same
// negative-money rejection the invoice path uses.
func subscriptionItemRows(subscriptionID uuid.UUID, entries []models.InvoiceItemInput) ([]models.SubscriptionItem, error) {
	items := make([]models.SubscriptionItem, 0, len(entries))
	for position, entry := range entries {
		if entry.Rate.IsNegative() {
			return nil, models.ErrNegativeMoney
		}
		items = append(items, models.SubscriptionItem{
			ID:             uuid.New(),
			SubscriptionID: subscriptionID,
			Description:    entry.Description,
			Quantity:       entry.Quantity,
			Rate:           entry.Rate,
			Position:       position,
		})
	}
	return items, nil
}

// rebindSubscriptionItems points a rebuilt item set at an existing subscription id.
func rebindSubscriptionItems(subscriptionID uuid.UUID, items []models.SubscriptionItem) []models.SubscriptionItem {
	for i := range items {
		items[i].SubscriptionID = subscriptionID
	}
	return items
}

// subscriptionInputError writes the 400 envelope for a bad payload/build. Every
// build failure (bad client id, bad date, negative money, send-without-client)
// is a malformed request, so it always maps to 400.
func subscriptionInputError(c fiber.Ctx, err error) error {
	return utils.Fail(c, fiber.StatusBadRequest, err.Error(), nil)
}

// CreateSubscription creates a subscription for the org.
// @Description Create a subscription.
// @Summary create subscription
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param request body models.SubscriptionInput true "Subscription payload"
// @Success 201 {object} map[string]interface{}
// @Security SessionCookie
// @Router /subscriptions [post]
func CreateSubscription(c fiber.Ctx) error {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	input := &models.SubscriptionInput{}
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
	sub, items, err := buildSubscriptionInput(input, orgID, utils.CurrentActorID(c))
	if err != nil {
		return subscriptionInputError(c, err)
	}
	if err := db.CreateSubscription(sub, items); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to create subscription", nil)
	}
	return utils.OK(c, fiber.StatusCreated, fiber.Map{"subscription": sub, "items": items})
}

// UpdateSubscription replaces a subscription and its items.
// @Description Update a subscription.
// @Summary update subscription
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param id path string true "Subscription ID"
// @Param request body models.SubscriptionInput true "Subscription payload"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /subscriptions/{id} [patch]
func UpdateSubscription(c fiber.Ctx) error {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid subscription id", nil)
	}
	input := &models.SubscriptionInput{}
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
	existing, err := db.GetSubscription(orgID, id)
	if err != nil {
		return utils.NotFoundOrFailed(c, err, "subscription")
	}
	sub, items, err := buildSubscriptionInput(input, orgID, existing.UserID)
	if err != nil {
		return subscriptionInputError(c, err)
	}
	sub.ID = existing.ID
	sub.CreatedAt = existing.CreatedAt
	// A blank start date keeps the existing cursor so an edit never unschedules
	// or re-fires an in-flight occurrence.
	if sub.NextRunDate == nil {
		sub.NextRunDate = existing.NextRunDate
	}
	sub.LastRunDate = existing.LastRunDate
	if err := db.UpdateSubscription(orgID, sub, rebindSubscriptionItems(existing.ID, items)); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to update subscription", nil)
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"subscription": sub, "items": items})
}

// DeleteSubscription deletes a subscription. It refuses (409) when the
// subscription already generated invoices; the owner pauses it instead.
// @Description Delete a subscription.
// @Summary delete subscription
// @Tags Subscriptions
// @Produce json
// @Param id path string true "Subscription ID"
// @Success 204 {string} status "ok"
// @Security SessionCookie
// @Router /subscriptions/{id} [delete]
func DeleteSubscription(c fiber.Ctx) error {
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
	if _, err := db.GetSubscription(orgID, id); err != nil {
		return utils.NotFoundOrFailed(c, err, "subscription")
	}
	runs, err := db.CountSubscriptionRuns(id)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load subscription runs", nil)
	}
	if runs > 0 {
		return utils.Fail(c, fiber.StatusConflict, "subscription has generated invoices; pause it instead", nil)
	}
	if err := db.DeleteSubscription(orgID, id); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to delete subscription", nil)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// UpdateSubscriptionStatus activates or pauses a subscription.
// @Description Update a subscription's status.
// @Summary update subscription status
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param id path string true "Subscription ID"
// @Param request body models.SubscriptionStatusInput true "Status payload"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /subscriptions/{id}/status [patch]
func UpdateSubscriptionStatus(c fiber.Ctx) error {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid subscription id", nil)
	}
	input := &models.SubscriptionStatusInput{}
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
	if _, err := db.GetSubscription(orgID, id); err != nil {
		return utils.NotFoundOrFailed(c, err, "subscription")
	}
	if err := db.UpdateSubscriptionStatus(orgID, id, input.Status); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to update subscription status", nil)
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"status": input.Status})
}
