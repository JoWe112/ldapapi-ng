package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	ldapclient "github.com/JoWe112/ldapapi-ng/internal/ldap"
)

// UserResponse describes the attributes returned for a user lookup.
// Attributes are returned as-is from LDAP (multi-valued).
// swagger:model UserResponse
type UserResponse struct {
	UID        string              `json:"uid" example:"jdoe"`
	DN         string              `json:"dn" example:"uid=jdoe,ou=people,dc=example,dc=org"`
	Attributes map[string][]string `json:"attributes"`
}

// User godoc
// @Summary      Fetch user attributes
// @Description  Returns the LDAP attributes of the user identified by :uid.
// @Tags         user
// @Produce      json
// @Param        uid  path      string  true  "User ID"
// @Success      200  {object}  UserResponse
// @Failure      404  {object}  ErrorBody
// @Failure      503  {object}  ErrorBody
// @Router       /v1/user/{uid} [get]
func User(ldap ldapclient.Client, log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uid")
		if uid == "" {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "uid path parameter is required.")
			return
		}

		attrs, err := ldap.LookupUser(c.Request.Context(), uid)
		if err != nil {
			if errors.Is(err, ldapclient.ErrUserNotFound) {
				writeError(c, http.StatusNotFound, "USER_NOT_FOUND", "The requested user was not found.")
				return
			}
			log.Error("ldap lookup failed", "uid", uid, "error", err.Error())
			writeError(c, http.StatusServiceUnavailable, "LDAP_UNAVAILABLE", "Directory backend is unavailable.")
			return
		}

		dn := ""
		if v := attrs["dn"]; len(v) > 0 {
			dn = v[0]
			delete(attrs, "dn")
		}

		c.JSON(http.StatusOK, UserResponse{
			UID:        uid,
			DN:         dn,
			Attributes: attrs,
		})
	}
}
