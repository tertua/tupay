package controllers

import (
	"database/sql"
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/pkg/utils"
)

// EnsureClientPortalLink get-or-creates the client's public portal link and
// returns its url_path. The raw token is returned once on a fresh mint; an
// existing live link returns an empty token (regenerate is the explicit
// rotate). Owner-only at the route level.
// @Description Ensure the public client portal link for a client.
// @Summary ensure client portal link
// @Tags Clients
// @Produce json
// @Param id path string true "Client ID"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /clients/{id}/portal [patch]
func EnsureClientPortalLink(c fiber.Ctx) error {
	orgID, clientID, ok := clientPortalScope(c)
	if !ok {
		return nil
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	if _, err := db.GetClient(orgID, clientID); err != nil {
		return utils.NotFoundOrFailed(c, err, "client")
	}
	_, raw, err := clientPortalLinkFor(*db, orgID, clientID, utils.CurrentActorID(c))
	if err != nil {
		return clientPortalFail(c, err)
	}
	return utils.OK(c, fiber.StatusOK, clientPortalLinkResponse(raw))
}

// RegenerateClientPortalLink rotates the client's link: the old token stops
// working immediately and the new raw token is returned once. Owner-only.
// @Description Regenerate the public client portal link for a client.
// @Summary regenerate client portal link
// @Tags Clients
// @Produce json
// @Param id path string true "Client ID"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /clients/{id}/portal/regenerate [post]
func RegenerateClientPortalLink(c fiber.Ctx) error {
	orgID, clientID, ok := clientPortalScope(c)
	if !ok {
		return nil
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	if _, err := db.GetClient(orgID, clientID); err != nil {
		return utils.NotFoundOrFailed(c, err, "client")
	}
	// No live link yet: mint one and hand back its token (a "regenerate" on a
	// client that never had a link is the same as creating it).
	if _, err := db.GetClientLinkForClient(orgID, clientID); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return clientPortalFail(c, err)
		}
		_, raw, err := clientPortalLinkFor(*db, orgID, clientID, utils.CurrentActorID(c))
		if err != nil {
			return clientPortalFail(c, err)
		}
		recordAudit(c, db, utils.CurrentActorID(c), "client.portal.regenerate", "client", clientID.String(), "")
		return utils.OK(c, fiber.StatusOK, clientPortalLinkResponse(raw))
	}
	raw, hash, err := newClientPortalToken()
	if err != nil {
		return clientPortalFail(c, err)
	}
	if err := db.RotateClientLink(orgID, clientID, hash); err != nil {
		return clientPortalFail(c, err)
	}
	recordAudit(c, db, utils.CurrentActorID(c), "client.portal.regenerate", "client", clientID.String(), "")
	return utils.OK(c, fiber.StatusOK, clientPortalLinkResponse(raw))
}

// RevokeClientPortalLink tombstones the client's link so it stops working
// immediately. Owner-only; idempotent (revoking an already-dead link is fine).
// @Description Revoke the public client portal link for a client.
// @Summary revoke client portal link
// @Tags Clients
// @Param id path string true "Client ID"
// @Success 204 {string} status "ok"
// @Security SessionCookie
// @Router /clients/{id}/portal [delete]
func RevokeClientPortalLink(c fiber.Ctx) error {
	orgID, clientID, ok := clientPortalScope(c)
	if !ok {
		return nil
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	if _, err := db.GetClient(orgID, clientID); err != nil {
		return utils.NotFoundOrFailed(c, err, "client")
	}
	if err := db.RevokeClientLink(orgID, clientID); err != nil {
		return clientPortalFail(c, err)
	}
	recordAudit(c, db, utils.CurrentActorID(c), "client.portal.revoke", "client", clientID.String(), "")
	return c.SendStatus(fiber.StatusNoContent)
}

// clientPortalScope resolves the tenant and parses :id, writing the shared
// stable error responses on failure (ok=false means the response is written).
func clientPortalScope(c fiber.Ctx) (orgID, clientID uuid.UUID, ok bool) {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		_ = utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
		return uuid.Nil, uuid.Nil, false
	}
	clientID, err = uuid.Parse(c.Params("id"))
	if err != nil {
		_ = utils.Fail(c, fiber.StatusBadRequest, "invalid client id", nil)
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, clientID, true
}

// clientPortalLinkResponse builds the owner payload; token is empty when the
// link already existed (url_path + active carry the state then).
func clientPortalLinkResponse(rawToken string) clientPortalResponse {
	return clientPortalResponse{ClientPortal: clientPortalLinkInfo{
		URLPath: clientPortalURLPath(rawToken),
		Token:   rawToken,
		Active:  true,
	}}
}

// clientPortalURLPath builds the FE path. Without a raw token the path is
// generic (the FE only renders the copy action when a token is present).
func clientPortalURLPath(rawToken string) string {
	if rawToken == "" {
		return "/client"
	}
	return "/client/" + rawToken
}

// clientPortalFail maps provisioning failures onto a stable 500 (the same
// message for every failure, so nothing internal leaks).
func clientPortalFail(c fiber.Ctx, _ error) error {
	return utils.Fail(c, fiber.StatusInternalServerError, "failed to provision client portal link", nil)
}
