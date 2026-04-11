package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginswagger "github.com/swaggo/gin-swagger"

	_ "github.com/JoWe112/ldapapi-ng/docs" // generated swagger spec
	"github.com/JoWe112/ldapapi-ng/internal/config"
	ldapclient "github.com/JoWe112/ldapapi-ng/internal/ldap"
)

// Router configures the HTTP routes and middleware for the API.
type Router struct {
	Config *config.Config
	LDAP   ldapclient.Client
	Log    *slog.Logger
}

// Build constructs and returns a *gin.Engine with all routes registered.
func (r *Router) Build() *gin.Engine {
	if !r.Config.DevMode {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(requestLogger(r.Log))

	// /health is always public so Kubernetes probes can reach it.
	engine.GET("/health", Health)

	// Swagger UI — only served when explicitly enabled (off in production by default).
	if r.Config.SwaggerEnabled {
		engine.GET("/swagger/*any", ginswagger.WrapHandler(swaggerfiles.Handler))
	}

	v1 := engine.Group("/v1")
	if r.Config.AuthMode == config.AuthModeStandalone {
		v1.Use(BasicAuth(r.LDAP))
	}

	v1.POST("/auth", Auth(r.LDAP, r.Log))
	v1.GET("/user/:uid", User(r.LDAP, r.Log))

	return engine
}
