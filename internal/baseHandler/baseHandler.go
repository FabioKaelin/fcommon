package baseHandler

import (
	"net/http"
	"time"

	"github.com/fabiokaelin/fcommon/internal/database"
	"github.com/fabiokaelin/fcommon/internal/logger"
	"github.com/fabiokaelin/fcommon/internal/values"
	"github.com/gin-gonic/gin"
)

func InitBaseHandler(router *gin.Engine, checkOAuthServer bool) {
	router.GET("/api/ping", pingHandler)
	router.GET("/api/version", versionHandler)
	router.GET("/internal/health/live", healthLiveHandler)
	router.GET("/internal/health/ready", healthReadyHandler(checkOAuthServer))
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
		c.AbortWithStatusJSON(500, gin.H{
			"error": "version not set",
		})
		return
	}
	c.IndentedJSON(200, gin.H{
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
	c.IndentedJSON(200, gin.H{
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
	c.IndentedJSON(200, gin.H{
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
func healthReadyHandler(checkOAuthServer bool) gin.HandlerFunc {
	return func(c *gin.Context) {

		if !checkURL(values.V.ImageServiceInternal + "/api/ping") {
			logger.Log.Error("Image Service Internal not ready")
			c.AbortWithStatusJSON(500, gin.H{
				"error": "Image Service Internal not ready",
			})
			return
		}

		if checkOAuthServer {
			if !checkURL(values.V.OAuthBackendInternal + "/internal/health/ready") {
				logger.Log.Error("OAuth Backend Internal not ready")
				c.AbortWithStatusJSON(500, gin.H{
					"error": "OAuth Backend Internal not ready",
				})
				return
			}
		}

		err := database.DBConnection.Ping()
		if err != nil {
			logger.Log.Error(err.Error())
			logger.Log.Error("database not ready")
			c.AbortWithStatusJSON(500, gin.H{
				"error": "database not ready",
			})
			return
		}

		c.IndentedJSON(200, gin.H{
			"status": "ok",
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
