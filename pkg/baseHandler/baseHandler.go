package baseHandler

import (
	"context"
	"net/http"
	"time"

	"github.com/fabiokaelin/fcommon/pkg/buildinfo"
	"github.com/fabiokaelin/fcommon/pkg/database"
	"github.com/fabiokaelin/fcommon/pkg/logger"
	"github.com/fabiokaelin/fcommon/pkg/values"
	"github.com/gin-gonic/gin"
)

type IgnoreChecks struct {
	OAuthServer bool
	ImageServer bool
	Database    bool
}

func InitBaseHandler(router *gin.Engine, ignored IgnoreChecks, defaultMessage string) {
	router.GET("/", defaultHandler(defaultMessage))
	router.GET("/api", defaultHandler(defaultMessage))

	router.GET("/api/ping", pingHandler)
	router.GET("/api/version", versionHandler)

	router.GET("/internal/health/live", healthLiveHandler)
	router.GET("/internal/health/ready", healthReadyHandler(ignored))
}

// versionHandler godoc
//
//	@Summary		Version
//	@Tags			Base
//	@Produce		json
//	@Success		200	{object}	map[string]any
//	@Failure		500	{object}	map[string]any
//	@Router			/api/version [get]
func versionHandler(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, buildinfo.Current(values.V.FVersion))
}

// pingHandler godoc
//
//	@Summary		Ping
//	@Tags			Base
//	@Produce		json
//	@Success		200	{object}	map[string]any
//	@Router			/api/ping [get]
func pingHandler(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, gin.H{
		"message": "pong",
	})
}

// healthLiveHandler godoc
//
//	@Summary		Live HealthCheck
//	@Tags			Base
//	@Produce		json
//	@Success		200	{object}	map[string]any
//	@Router			/internal/health/live [get]
func healthLiveHandler(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// healthReadyHandler godoc
//
//	@Summary		Ready HealthCheck
//	@Tags			Base
//	@Produce		json
//	@Success		200	{object}	map[string]any
//	@Failure		500	{object}	map[string]any
//	@Router			/internal/health/ready [get]
func healthReadyHandler(ignored IgnoreChecks) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()

		if !ignored.ImageServer {
			if !checkURL(ctx, values.V.ImageServiceInternal+"/api/ping") {
				logger.Log.Error("Image Service Internal not ready")
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Image Service Internal not ready",
				})
				return
			}
		}

		if !ignored.OAuthServer {
			if !checkURL(ctx, values.V.OAuthBackendInternal+"/internal/health/ready") {
				logger.Log.Error("OAuth Backend Internal not ready")
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "OAuth Backend Internal not ready",
				})
				return
			}
		}

		if !ignored.Database {
			if database.DBConnection == nil {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Database not ready"})
				return
			}
			err := database.DBConnection.PingContext(ctx)
			if err != nil {
				logger.Log.Error(err.Error())
				logger.Log.Error("database not ready")
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "database not ready",
				})
				return
			}
		}

		c.IndentedJSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	}
}

// defaultHandler godoc
//
//	@Summary		defaultHandler
//	@Description	defaultHandler
//	@Tags			default
//	@Produce		json
//	@Success		200	{string}	string
//	@Router			/ [get]
//	@Router			/api [get]
func defaultHandler(message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": message,
		})
	}
}

// checkURL sends a GET request and checks if the response is 200 OK
func checkURL(ctx context.Context, url string) bool {
	client := http.Client{
		Timeout: 3 * time.Second, // Set a timeout to avoid hanging requests
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		logger.Log.Error(err.Error())
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
