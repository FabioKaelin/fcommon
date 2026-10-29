package cors

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestCredentialedOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, origin := range []string{"https://attacker.example", "https://fabkli.ch.attacker.example", "https://attackerfabkli.ch", "null", "http://tipp.fabkli.ch", "https://oauth.fabkli.ch/path", "https://user@oauth.fabkli.ch", "https://oauth.fabkli.ch:444", "https://oauth.fabkli.ch?x=1", "https://oauth.fabkli.ch#x", "https://oauth.fabkli.ch", "https://fabkli.ch", ""} {
		t.Run(origin, func(t *testing.T) {
			r := gin.New()
			r.Use(CORSMiddleware())
			r.GET("/", func(c *gin.Context) { c.Status(200) })
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "https://tipp.fabkli.ch/", nil)
			req.Header.Set("Origin", origin)
			r.ServeHTTP(w, req)
			accepted := origin == "https://oauth.fabkli.ch" || origin == "https://fabkli.ch" || origin == ""
			if accepted && w.Code != 200 || !accepted && w.Code != 403 {
				t.Fatalf("origin %q status %d", origin, w.Code)
			}
			if !accepted && w.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("rejected origin reflected")
			}
		})
	}
	r := gin.New()
	r.Use(CORSMiddleware("http://localhost:5173"))
	r.GET("/", func(c *gin.Context) { c.Status(200) })
	for _, origin := range []string{"http://localhost:5173", "https://tipp.fabkli.ch"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", origin)
		r.ServeHTTP(w, req)
		if (w.Code == 200) != (origin == "http://localhost:5173") {
			t.Fatal("explicit list not enforced")
		}
	}
}
