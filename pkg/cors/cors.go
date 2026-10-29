// Package cors protects credentialed browser access to FabKli APIs.
package cors

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"net/url"
	"strings"
)

// CORSMiddleware defaults to the owner's HTTPS FabKli origins. Optional exact
// origins support deliberate external clients or local development. It never
// reflects an arbitrary origin with credentials or treats CORS as authentication.
func CORSMiddleware(allowedOrigins ...string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, origin := range allowedOrigins {
		allowed[origin] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		c.Writer.Header().Add("Vary", "Origin")
		if origin != "" {
			u, err := url.Parse(origin)
			safe := err == nil && u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == "" && u.Port() == "" && u.Scheme == "https" && (u.Hostname() == "fabkli.ch" || strings.HasSuffix(u.Hostname(), ".fabkli.ch"))
			if len(allowed) > 0 {
				safe = allowed[origin]
			}
			if !safe {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, token, X-CSRF-Token, X-Requested-With")
			c.Header("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
