package controllers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/gateway"
	"github.com/tertua/tupay/platform/relay"
)

// @Description Register a downstream project.
// @Summary create gateway project
// @Tags Admin
// @Accept json
// @Produce json
// @Param request body models.CreateProjectInput true "Project payload"
// @Success 201 {object} map[string]interface{}
// @Router /admin/gateway/projects [post]
func CreateProject(c fiber.Ctx) error {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	ownerID := utils.CurrentActorID(c)
	input := &models.CreateProjectInput{}
	if err := c.Bind().Body(input); err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid request body", nil)
	}
	input.Slug = strings.TrimSpace(strings.ToLower(input.Slug))
	if !slugPattern.MatchString(input.Slug) {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid slug, use lowercase letters, numbers and dashes", nil)
	}
	if err := utils.NewValidator().Struct(input); err != nil {
		return utils.ValidationFailed(c, err)
	}
	if !validateEndpointURL(strings.TrimSpace(input.WebhookURL)) {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid webhook_url, must be http(s) and reachable", nil)
	}
	defaultGateway := resolveGateway(input.DefaultGateway, gateway.DefaultProvider())
	if _, err := gateway.Get(defaultGateway); err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "unknown payment gateway", nil)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	if _, err := db.GetProjectBySlug(input.Slug); err == nil {
		return utils.Fail(c, fiber.StatusConflict, "project slug already exists", nil)
	}
	apiKey, err := relay.GenerateAPIKey()
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to generate api key", nil)
	}
	secret, err := relay.GenerateSecret()
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to generate secret", nil)
	}
	now := time.Now()
	p := &models.GatewayProject{
		Slug:           input.Slug,
		OwnerUserID:    &ownerID,
		OrgID:          orgID,
		Name:           strings.TrimSpace(input.Name),
		APIKeyHash:     relay.HashKey(apiKey),
		WebhookURL:     strings.TrimSpace(input.WebhookURL),
		WebhookSecret:  secret,
		DefaultGateway: defaultGateway,
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := db.CreateProject(p); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to create project", nil)
	}
	return utils.OK(c, fiber.StatusCreated, fiber.Map{"project": projectResponse(*p, true, apiKey)})
}

// ListProjects returns one page of downstream projects for admins.
// @Description List gateway projects.
// @Summary list gateway projects
// @Tags Admin
// @Produce json
// @Param page query int false "Page number (default 1)"
// @Param per_page query int false "Items per page (default 20, max 100)"
// @Success 200 {object} map[string]interface{}
// @Router /admin/gateway/projects [get]
func ListProjects(c fiber.Ctx) error {
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	paging := utils.ParsePagination(c)
	rows, err := db.ListProjects(paging.Limit(), paging.Offset())
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load projects", nil)
	}
	total, err := db.CountProjects()
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to count projects", nil)
	}
	out := make([]projectResponseRow, 0, len(rows))
	for _, p := range rows {
		out = append(out, projectResponse(p, true, ""))
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"projects": out, "meta": paging.Meta(total)})
}

// UpdateProject edits project metadata.
// @Description Update a gateway project.
// @Summary update gateway project
// @Tags Admin
// @Accept json
// @Produce json
// @Param slug path string true "Project slug"
// @Param request body models.UpdateProjectInput true "Project payload"
// @Success 200 {object} map[string]interface{}
// @Router /admin/gateway/projects/{slug} [patch]
func UpdateProject(c fiber.Ctx) error {
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	p, err := db.GetProjectBySlug(c.Params("slug"))
	if err != nil {
		return utils.NotFoundOrFailed(c, err, "project")
	}
	input := &models.UpdateProjectInput{}
	if err := c.Bind().Body(input); err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid request body", nil)
	}
	if err := utils.NewValidator().Struct(input); err != nil {
		return utils.ValidationFailed(c, err)
	}
	if strings.TrimSpace(input.Name) != "" {
		p.Name = strings.TrimSpace(input.Name)
	}
	if strings.TrimSpace(input.WebhookURL) != "" {
		target := strings.TrimSpace(input.WebhookURL)
		// Same guard as create: the outbox worker POSTs to this URL, so a
		// scheme-only or internal target must never be stored (the owner could
		// otherwise point us at loopback or the metadata endpoint).
		if !validateEndpointURL(target) {
			return utils.Fail(c, fiber.StatusBadRequest, "invalid webhook_url, must be http(s) and reachable", nil)
		}
		p.WebhookURL = target
	}
	if strings.TrimSpace(input.DefaultGateway) != "" {
		name := resolveGateway(input.DefaultGateway, "")
		if _, err := gateway.Get(name); err != nil {
			return utils.Fail(c, fiber.StatusBadRequest, "unknown payment gateway", nil)
		}
		p.DefaultGateway = name
	}
	if input.IsActive != nil {
		p.IsActive = *input.IsActive
	}
	p.UpdatedAt = time.Now()
	if err := db.SaveProject(&p); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to update project", nil)
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"project": projectResponse(p, true, "")})
}

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
	return utils.OK(c, fiber.StatusOK, fiber.Map{"project": projectResponse(p, true, apiKey)})
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
	p.WebhookSecret = secret
	p.UpdatedAt = time.Now()
	if err := db.SaveProject(&p); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to rotate secret", nil)
	}
	if adminID := utils.CurrentActorID(c); adminID != uuid.Nil {
		recordAudit(c, db, adminID, "gateway.secret.rotate", "project", p.Slug, "")
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"project": projectResponse(p, true, "")})
}
