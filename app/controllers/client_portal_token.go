package controllers

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/platform/database"
)

// newClientPortalToken mints a public client-portal token: 24 random bytes →
// 48 hex chars, with the sha256 hex (64 chars) stored at rest. The raw token
// is returned to the owner exactly once; only the hash ever reaches the DB.
func newClientPortalToken() (raw, hash string, err error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(buf)
	return raw, hashClientPortalToken(raw), nil
}

// hashClientPortalToken is the single hashing function for client-portal
// tokens, used on both the mint and the lookup path so they can never diverge.
func hashClientPortalToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// clientPortalTokenError marks a failure in the token-mint/link write path so
// callers can map it onto a stable 500 without leaking internals.
var errClientPortalToken = fmt.Errorf("failed to provision client portal link")

// clientPortalLinkFor returns the live link for a client, minting one when
// none exists. The raw token is non-empty ONLY when the link was freshly
// minted; an existing live link returns an empty raw token (the stored hash
// is never re-exposed). Mirrors ensurePaymentLink's idempotency.
func clientPortalLinkFor(db database.Queries, orgID, clientID, userID uuid.UUID) (link models.ClientLink, rawToken string, err error) {
	existing, err := db.GetClientLinkForClient(orgID, clientID)
	if err == nil {
		return existing, "", nil
	}
	if err != sql.ErrNoRows {
		return models.ClientLink{}, "", fmt.Errorf("%w: %v", errClientPortalToken, err)
	}
	raw, hash, err := newClientPortalToken()
	if err != nil {
		return models.ClientLink{}, "", fmt.Errorf("%w: %v", errClientPortalToken, err)
	}
	link = models.ClientLink{
		TokenHash: hash,
		ClientID:  clientID,
		OrgID:     orgID,
		UserID:    userID,
		CreatedAt: time.Now(),
	}
	if err := db.CreateClientLink(&link); err != nil {
		return models.ClientLink{}, "", fmt.Errorf("%w: %v", errClientPortalToken, err)
	}
	return link, raw, nil
}
