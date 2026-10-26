package middleware

import (
	"errors"
	"time"

	"github.com/tertua/tupay/pkg/configs"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/cache"
	"github.com/tertua/tupay/platform/database"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AuthRequired protects routes with a cookie (or Bearer) JWT session.
// Expired access tokens are refreshed transparently via the refresh cookie,
// so the frontend never needs token handling logic. Sessions are strict
// single-session: a new login mints a new sid and the previous access and
// refresh tokens stop working immediately.
func AuthRequired() fiber.Handler {
	return func(c fiber.Ctx) error {
		if userID, sid, ok := validAccessToken(accessTokenString(c)); ok && sessionMatches(c, userID, sid) {
			c.Locals(utils.SessionUserIDKey, userID)
			return c.Next()
		}

		userID, ok := refreshSession(c)
		if !ok {
			return clearAuthCookies(c)
		}

		c.Locals(utils.SessionUserIDKey, userID)
		return c.Next()
	}
}

// accessTokenString reads the access token from cookie or Authorization header.
func accessTokenString(c fiber.Ctx) string {
	if token := c.Cookies(utils.AccessCookieName); token != "" {
		return token
	}
	if parts := splitBearer(c.Get("Authorization")); parts != "" {
		return parts
	}
	return ""
}

// validAccessToken parses and validates an access token.
func validAccessToken(tokenString string) (userID uuid.UUID, sid string, ok bool) {
	if tokenString == "" {
		return uuid.Nil, "", false
	}

	token, err := jwt.Parse(tokenString, jwtKeyFunc)
	if err != nil || !token.Valid {
		return uuid.Nil, "", false
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, "", false
	}

	rawID, ok := claims["id"].(string)
	if !ok {
		return uuid.Nil, "", false
	}
	parsedID, err := uuid.Parse(rawID)
	if err != nil {
		return uuid.Nil, "", false
	}

	sid, _ = claims["sid"].(string)
	return parsedID, sid, true
}

// sessionMatches reports whether the token's sid is the live session.
// Pre-sid deployments stored a bare refresh string: Decode yields an empty
// sid for those, which only matches a sid-less token; the next transparent
// refresh rewrites the entry in the new format.
func sessionMatches(c fiber.Ctx, userID uuid.UUID, sid string) bool {
	store, err := cache.Sessions()
	if err != nil {
		return false
	}
	stored, err := store.Get(c.Context(), userID.String())
	if err != nil {
		return false
	}
	storedSID, _, _, _, ok := cache.DecodeSessionValue(stored)
	if !ok {
		return false
	}
	if storedSID == "" {
		return sid == ""
	}
	return sid != "" && sid == storedSID
}

// refreshSession issues a new token pair using the refresh cookie.
// The access cookie outlives the JWT inside it (see IssueSession), so the
// browser keeps sending the expired token as an identity hint long after
// its 15-minute cryptographic expiry — the refresh token then renews the
// session transparently without forcing a re-login.
func refreshSession(c fiber.Ctx) (uuid.UUID, bool) {
	accessString := accessTokenString(c)
	refreshString := c.Cookies(utils.RefreshCookieName)
	if refreshString == "" {
		return uuid.Nil, false
	}

	userID, tokenSID, ok := userIDFromAccess(accessString)
	if !ok {
		return uuid.Nil, false
	}

	// Check refresh token expiry.
	expiresRefresh, err := utils.ParseRefreshToken(refreshString)
	if err != nil || time.Now().Unix() >= expiresRefresh {
		return uuid.Nil, false
	}

	// Check refresh token against the session store.
	store, err := cache.Sessions()
	if err != nil {
		return uuid.Nil, false
	}
	stored, err := store.Get(c.Context(), userID.String())
	if err != nil {
		return uuid.Nil, false
	}
	storedSID, storedRefresh, storedCSRF, storedOrg, ok := cache.DecodeSessionValue(stored)
	if !ok || !secureEqual(storedRefresh, refreshString) {
		return uuid.Nil, false
	}
	if storedSID == "" {
		// Legacy bare-refresh entry: only a sid-less token matches, and
		// the refresh below rewrites the entry in the new format.
		if tokenSID != "" {
			return uuid.Nil, false
		}
	} else if tokenSID != "" && tokenSID != storedSID {
		// A kicked previous session presenting the live refresh with its
		// own stale access hint is still rejected.
		return uuid.Nil, false
	}

	// Check user still exists.
	db, err := database.OpenDBConnection()
	if err != nil {
		return uuid.Nil, false
	}
	if _, err := db.GetUserByID(userID); err != nil {
		return uuid.Nil, false
	}

	// Issue new tokens keeping the session id (transparent refresh must
	// not kick sibling tabs sharing the session) and persist them. The
	// bound CSRF token survives: refresh is not a privilege moment.
	tokens, err := utils.IssueSession(c, userID, storedSID)
	if err != nil {
		return uuid.Nil, false
	}
	if err := store.Set(c.Context(), userID.String(), cache.EncodeSessionValue(tokens.SID, tokens.Refresh, storedCSRF, storedOrg), cache.RefreshTTL()); err != nil {
		return uuid.Nil, false
	}

	return userID, true
}

// userIDFromAccess recovers the user ID and session id from an access token
// without expiry validation. Only expiry is recoverable: malformed or
// badly-signed tokens always fail, and an empty input (no cookie/header at
// all) fails — the refresh token carries no identity, so there is nothing
// to look up. Pre-sid tokens yield an empty sid.
func userIDFromAccess(accessString string) (userID uuid.UUID, sid string, ok bool) {
	if accessString == "" {
		return uuid.Nil, "", false
	}
	if _, err := jwt.Parse(accessString, jwtKeyFunc); err != nil {
		if !errors.Is(err, jwt.ErrTokenExpired) {
			return uuid.Nil, "", false
		}
	}
	parser := jwt.NewParser()
	unverified, _, err := parser.ParseUnverified(accessString, jwt.MapClaims{})
	if err != nil {
		return uuid.Nil, "", false
	}
	claims, ok := unverified.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, "", false
	}
	id, ok := claims["id"].(string)
	if !ok {
		return uuid.Nil, "", false
	}
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, "", false
	}
	sid, _ = claims["sid"].(string)
	return parsedID, sid, true
}

// splitBearer extracts the token from a "Bearer <token>" header value.
func splitBearer(header string) string {
	const prefix = "Bearer "
	if len(header) > len(prefix) && header[:len(prefix)] == prefix {
		return header[len(prefix):]
	}
	return ""
}

// jwtKeyFunc returns the JWT signing key, accepting only HS256.
// Without the alg pin a token signed with another algorithm (e.g. "none"
// on a permissive parser) could slip through key confusion.
func jwtKeyFunc(token *jwt.Token) (any, error) {
	if token.Method != jwt.SigningMethodHS256 {
		return nil, errors.New("unexpected signing method")
	}
	return []byte(configs.Get().JWT.Secret), nil
}
