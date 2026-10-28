package controllers

import (
	"time"

	"github.com/tertua/tupay/app/models"
)

// projectResponse renders a project for the admin API. revealSecret carries the
// raw webhook secret only on the create and rotate-secret responses: the secret
// is sealed at rest, so the stored value is never echoed back.
func projectResponse(p models.GatewayProject, revealSecret, apiKey string) projectResponseRow {
	out := projectResponseRow{
		Slug:           p.Slug,
		Name:           p.Name,
		WebhookURL:     p.WebhookURL,
		DefaultGateway: p.DefaultGateway,
		IsActive:       p.IsActive,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
	// A pointer keeps webhook_secret present even when the revealed value is the
	// empty string; omitempty on a plain string would drop it.
	if revealSecret != "" {
		out.WebhookSecret = &revealSecret
	}
	if apiKey != "" {
		out.APIKey = apiKey
	}
	return out
}

type projectResponseRow struct {
	Slug           string    `json:"slug"`
	Name           string    `json:"name"`
	WebhookURL     string    `json:"webhook_url"`
	DefaultGateway string    `json:"default_gateway"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	WebhookSecret  *string   `json:"webhook_secret,omitempty"`
	APIKey         string    `json:"api_key,omitempty"`
}
