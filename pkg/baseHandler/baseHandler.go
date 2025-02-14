package baseHandler

import (
	"net/http"
	"time"

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
	if values.V.FVersion == "" {
		logger.Log.Error("version not set")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "version not set",
		})
		return
	}
	c.IndentedJSON(http.StatusOK, gin.H{
		"version": values.V.FVersion,
	})
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

		if !ignored.ImageServer {
			if !checkURL(values.V.ImageServiceInternal + "/api/ping") {
				logger.Log.Error("Image Service Internal not ready")
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Image Service Internal not ready",
				})
				return
			}
		}

		if !ignored.OAuthServer {
			if !checkURL(values.V.OAuthBackendInternal + "/internal/health/ready") {
				logger.Log.Error("OAuth Backend Internal not ready")
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "OAuth Backend Internal not ready",
				})
				return
			}
		}

		if !ignored.Database {
			err := database.DBConnection.Ping()
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
func checkURL(url string) bool {
	client := http.Client{
		Timeout: 3 * time.Second, // Set a timeout to avoid hanging requests
	}

	resp, err := client.Get(url)
	if err != nil {
		logger.Log.Error(err.Error())
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
