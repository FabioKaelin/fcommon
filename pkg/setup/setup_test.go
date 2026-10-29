package setup

import (
	"github.com/gin-gonic/gin"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxyConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if _, err := New(Options{TrustedProxies: []string{"localhost"}}); err == nil {
		t.Fatal("invalid proxy silently accepted")
	}
	r, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	r.GET("/ip", func(c *gin.Context) { c.String(200, c.ClientIP()) })
	w := httptest.NewRecorder()
	q := httptest.NewRequest("GET", "https://fabkli.ch/ip", nil)
	q.RemoteAddr = "192.0.2.1:111"
	q.Header.Set("X-Forwarded-For", "198.51.100.2")
	r.ServeHTTP(w, q)
	if w.Body.String() != "192.0.2.1" {
		t.Fatal("untrusted forwarded address accepted")
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing content-type protection")
	}
}

func TestConfiguredBodyLimit(t *testing.T) {
	r, err := New(Options{MaxBodyBytes: 16})
	if err != nil {
		t.Fatal(err)
	}
	r.POST("/body", func(c *gin.Context) {
		if _, err := io.Copy(io.Discard, c.Request.Body); err != nil {
			c.Status(413)
		} else {
			c.Status(200)
		}
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/body", strings.NewReader(strings.Repeat("x", 17))))
	if w.Code != 413 {
		t.Fatal("body limit bypassed")
	}
}
