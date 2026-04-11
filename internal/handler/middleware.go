package handler

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	ldapclient "github.com/JoWe112/ldapapi-ng/internal/ldap"
)

// BasicAuth returns a middleware that validates HTTP Basic Auth credentials
// by performing an LDAP bind. Used in standalone auth mode.
//
// In gateway mode this middleware is not installed — the upstream gateway
// (KrakenD) handles authentication and a NetworkPolicy restricts ingress.
func BasicAuth(ldap ldapclient.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, pass, ok := parseBasicAuth(c.GetHeader("Authorization"))
		if !ok {
			c.Header("WWW-Authenticate", `Basic realm="ldapapi-ng"`)
			writeError(c, http.StatusUnauthorized, "MISSING_CREDENTIALS", "Basic authentication is required.")
			return
		}

		if err := ldap.Authenticate(c.Request.Context(), user, pass); err != nil {
			if errors.Is(err, ldapclient.ErrInvalidCredentials) {
				writeError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "The provided credentials are invalid.")
				return
			}
			writeError(c, http.StatusServiceUnavailable, "LDAP_UNAVAILABLE", "Authentication backend is unavailable.")
			return
		}

		// Propagate the authenticated username to downstream handlers.
		c.Set("auth.user", user)
		c.Next()
	}
}

// parseBasicAuth decodes an "Authorization: Basic <base64>" header.
func parseBasicAuth(header string) (user, pass string, ok bool) {
	const prefix = "Basic "
	if !strings.HasPrefix(header, prefix) {
		return "", "", false
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(header, prefix))
	if err != nil {
		return "", "", false
	}
	i := strings.IndexByte(string(raw), ':')
	if i < 0 {
		return "", "", false
	}
	return string(raw[:i]), string(raw[i+1:]), true
}
