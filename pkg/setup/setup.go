// Package setup centralizes secure, predictable Gin API initialization.
package setup

import (
	"github.com/fabiokaelin/fcommon/pkg/baseHandler"
	"github.com/fabiokaelin/fcommon/pkg/cors"
	"github.com/gin-gonic/gin"
	"net/http"
)

type Options struct {
	// MaxBodyBytes bounds write bodies when positive; leave zero for streaming APIs.
	MaxBodyBytes   int64
	Message        string
	IgnoreChecks   baseHandler.IgnoreChecks
	AllowedOrigins []string
	// TrustedProxies must contain IP addresses or CIDRs; an empty list trusts none.
	TrustedProxies []string
	Middleware     []gin.HandlerFunc
}

func New(options Options) (*gin.Engine, error) {
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.ForwardedByClientIP = len(options.TrustedProxies) > 0
	if err := router.SetTrustedProxies(options.TrustedProxies); err != nil {
		return nil, err
	}
	router.Use(gin.Recovery())
	router.Use(func(c *gin.Context) { c.Header("X-Content-Type-Options", "nosniff"); c.Next() })
	router.Use(cors.CORSMiddleware(options.AllowedOrigins...))
	if options.MaxBodyBytes > 0 {
		router.Use(func(c *gin.Context) {
			if c.Request.Body != nil {
				c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, options.MaxBodyBytes)
			}
			c.Next()
		})
	}
	router.Use(options.Middleware...)
	baseHandler.InitBaseHandler(router, options.IgnoreChecks, options.Message)
	return router, nil
}
