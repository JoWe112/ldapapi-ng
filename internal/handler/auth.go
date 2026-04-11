package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	ldapclient "github.com/JoWe112/ldapapi-ng/internal/ldap"
)

// AuthResponse is returned on a successful LDAP bind.
// swagger:model AuthResponse
type AuthResponse struct {
	Authenticated bool   `json:"authenticated" example:"true"`
	Username      string `json:"username" example:"jdoe"`
}

// Auth godoc
// @Summary      Authenticate a user against LDAP
// @Description  Performs an LDAP bind using the credentials supplied via HTTP Basic Auth.
// @Description  The request body is intentionally empty — credentials are sent in the Authorization header.
// @Tags         auth
// @Produce      json
// @Security     BasicAuth
// @Success      200  {object}  AuthResponse
// @Failure      401  {object}  ErrorBody
// @Failure      503  {object}  ErrorBody
// @Router       /v1/auth [post]
func Auth(ldap ldapclient.Client, log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, pass, ok := parseBasicAuth(c.GetHeader("Authorization"))
		if !ok {
			c.Header("WWW-Authenticate", `Basic realm="ldapapi-ng"`)
			writeError(c, http.StatusUnauthorized, "MISSING_CREDENTIALS", "Basic authentication is required.")
			return
		}

		if err := ldap.Authenticate(c.Request.Context(), user, pass); err != nil {
			if errors.Is(err, ldapclient.ErrInvalidCredentials) {
				log.Info("authentication failed", "user", user)
				writeError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "The provided credentials are invalid.")
				return
			}
			log.Error("ldap backend error", "error", err.Error())
			writeError(c, http.StatusServiceUnavailable, "LDAP_UNAVAILABLE", "Authentication backend is unavailable.")
			return
		}

		log.Info("authentication succeeded", "user", user)
		c.JSON(http.StatusOK, AuthResponse{Authenticated: true, Username: user})
	}
}
