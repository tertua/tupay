package controllers

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/relay"
)

// @Description Rotate a project API key.
// @Summary rotate project key
// @Tags Admin
// @Produce json
// @Param slug path string true "Project slug"
// @Success 200 {object} map[string]interface{}
// @Router /admin/gateway/projects/{slug}/rotate-key [post]
func RotateProjectKey(c fiber.Ctx) error {
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	p, err := db.GetProjectBySlug(c.Params("slug"))
	if err != nil {
		return utils.NotFoundOrFailed(c, err, "project")
	}
	apiKey, err := relay.GenerateAPIKey()
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to generate api key", nil)
	}
	p.APIKeyHash = relay.HashKey(apiKey)
	p.UpdatedAt = time.Now()
	if err := db.SaveProject(&p); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to rotate key", nil)
	}
	if adminID := utils.CurrentActorID(c); adminID != uuid.Nil {
		recordAudit(c, db, adminID, "gateway.key.rotate", "project", p.Slug, "")
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"project": projectResponse(p, "", apiKey)})
}

// @Description Rotate a project webhook secret.
// @Summary rotate project secret
// @Tags Admin
// @Produce json
// @Param slug path string true "Project slug"
// @Success 200 {object} map[string]interface{}
// @Router /admin/gateway/projects/{slug}/rotate-secret [post]
func RotateProjectSecret(c fiber.Ctx) error {
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	p, err := db.GetProjectBySlug(c.Params("slug"))
	if err != nil {
		return utils.NotFoundOrFailed(c, err, "project")
	}
	secret, err := relay.GenerateSecret()
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to generate secret", nil)
	}
	// The raw secret is revealed exactly once, here; only the sealed form is stored.
	sealed, err := relay.SealSecret(secret)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to protect webhook secret", nil)
	}
	p.WebhookSecret = sealed
	p.UpdatedAt = time.Now()
	if err := db.SaveProject(&p); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to rotate secret", nil)
	}
	if adminID := utils.CurrentActorID(c); adminID != uuid.Nil {
		recordAudit(c, db, adminID, "gateway.secret.rotate", "project", p.Slug, "")
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"project": projectResponse(p, secret, "")})
}
