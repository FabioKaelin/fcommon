package oauthHandler

import (
	"github.com/fabiokaelin/fcommon/pkg/values"
	"github.com/gin-gonic/gin"
)

// InitOAuthRouter defines the routes for the oauth
func InitOAuthRouter(apiGroup *gin.RouterGroup) {
	oauthGroup := apiGroup.Group("/oauth")
	{
		oauthGroup.GET("", oauthGet)
		oauthGroup.GET("/backend", oauthGetBackend)
	}
}

// oauthGet godoc
//
//	@Summary		Get oauth url
//	@Description	Return the oauth frontend url
//	@Tags			oauth
//	@Produce		json
//	@Success		200	{string}	url
//	@Router			/oauth [get]
func oauthGet(c *gin.Context) {
	c.IndentedJSON(200, gin.H{
		"url": values.V.OAuthFrontendServer,
	})
}

// oauthGetBackend godoc
//
//	@Summary		Get oauth backend url
//	@Description	Return the oauth backend url
//	@Tags			oauth
//	@Produce		json
//	@Success		200	{string}	url
//	@Router			/oauth/backend [get]
func oauthGetBackend(c *gin.Context) {
	c.IndentedJSON(200, gin.H{
		"url": values.V.OAuthBackendServer,
	})
}
