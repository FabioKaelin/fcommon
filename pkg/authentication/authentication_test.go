package authentication

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fabiokaelin/fcommon/pkg/oauthsession"
	"github.com/fabiokaelin/fcommon/pkg/users"
	"github.com/fabiokaelin/fcommon/pkg/values"
	"github.com/fabiokaelin/ferror"
	"github.com/gin-gonic/gin"
)

func TestBrowserActivityForwardsRenewal(t *testing.T) {
	oldValues, oldClient := values.V, sessionClient
	oldExists, oldGet, oldCreate := userExist, getUserByID, createUser
	defer func() {
		values.V = oldValues
		sessionClient = oldClient
		userExist = oldExists
		getUserByID = oldGet
		createUser = oldCreate
	}()
	SetRequiredFunctions(func(string) bool { return true }, func(id string) (users.Minimal, ferror.FError) { return users.Minimal{ID: id, Privileges: 13}, nil }, func(users.Minimal) ferror.FError { t.Fatal("existing user should not be created"); return nil })
	status := 200
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/users/me" {
			t.Error("incorrect auth endpoint")
		}
		w.Header().Add("Set-Cookie", "token=opaque; Domain=fabkli.ch; Path=/; Max-Age=2592000; Secure; HttpOnly; SameSite=Lax")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"id":"owner","name":"Owner","email":"owner@example.com"}`))
	}))
	defer auth.Close()
	values.V.OAuthBackendInternal = auth.URL
	sessionClient = oauthsession.NewClient()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	calls := 0
	router.GET("/api/users/me", CheckLoginRequest(), func(ctx *gin.Context) { calls++; ctx.JSON(200, ctx.MustGet("user")) })
	for _, c := range []struct {
		name, cookie          string
		auth, status, cookies int
	}{
		{"valid activity", "token=opaque", 200, 200, 1},
		{"revoked", "token=opaque", 401, 401, 0},
		{"provider failure", "token=opaque", 500, 503, 0},
		{"duplicate credentials", "token=opaque; token=evil", 200, 401, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			status = c.auth
			before := calls
			req := httptest.NewRequest("GET", "https://tipp.fabkli.ch/api/users/me", nil)
			req.Header.Set("Cookie", c.cookie)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != c.status {
				t.Fatalf("got %d want %d", w.Code, c.status)
			}
			if len(w.Result().Cookies()) != c.cookies {
				t.Error("renewal cookie not forwarded correctly")
			}
			if c.status != 200 && calls != before {
				t.Error("failed auth reached handler")
			}
		})
	}
}

func TestCookieWritesRejectCrossOriginBeforeAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/write", CheckLoginRequest(), func(c *gin.Context) { t.Fatal("unsafe request reached handler") })
	for _, origin := range []string{"https://evil.example", "https://other.fabkli.ch", ""} {
		req := httptest.NewRequest("POST", "https://app.fabkli.ch/write", nil)
		req.AddCookie(&http.Cookie{Name: "token", Value: "opaque"})
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		out := httptest.NewRecorder()
		router.ServeHTTP(out, req)
		if out.Code != 403 {
			t.Fatalf("unsafe origin returned %d", out.Code)
		}
	}
}
