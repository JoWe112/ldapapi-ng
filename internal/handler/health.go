package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/JoWe112/ldapapi-ng/internal/version"
)

// HealthResponse is the body returned by GET /health.
// swagger:model HealthResponse
type HealthResponse struct {
	Status  string `json:"status" example:"ok"`
	Version string `json:"version" example:"0.1.0"`
	Commit  string `json:"commit" example:"a1b2c3d"`
}

// Health godoc
// @Summary      Health check
// @Description  Returns the service status along with build version and commit.
// @Tags         system
// @Produce      json
// @Success      200  {object}  HealthResponse
// @Router       /health [get]
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:  "ok",
		Version: version.Version,
		Commit:  version.Commit,
	})
}
