package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func fixture(t *testing.T) (*Engine, *miniredis.Miniredis) {
	t.Helper()
	db := miniredis.RunT(t)
	db.SetTime(time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC))
	client := redis.NewClient(&redis.Options{Addr: db.Addr(), MaxRetries: -1, PoolSize: 4, DialTimeout: time.Second, ContextTimeoutEnabled: true})
	t.Cleanup(func() { client.Close() })
	return &Engine{Store: &Store{Client: client, Namespace: "test", Secret: []byte(strings.Repeat("s", 32))}, Mode: "enforce", EdgeKey: strings.Repeat("e", 32), ServiceKey: strings.Repeat("i", 32), AuthenticatedPerMinute: 1200, AuthenticatedBurst: 120, local: fallback{now: time.Now}}, db
}
func TestSharedConcurrentRollingQuotaAndExpiry(t *testing.T) {
	e, db := fixture(t)
	second := &Store{Client: e.Store.Client, Namespace: e.Store.Namespace, Secret: e.Store.Secret}
	var successes atomic.Int32
	var wg sync.WaitGroup
	rule := Window("attempts", "same-user", 5, time.Minute)
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := e.Store
			if i%2 == 0 {
				s = second
			}
			d, err := s.Check(context.Background(), rule)
			if err != nil {
				t.Error(err)
			}
			if d.Allowed {
				successes.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if successes.Load() != 5 {
		t.Fatalf("concurrent replicas admitted%d, want5", successes.Load())
	}
	d, err := second.Check(context.Background(), rule)
	if err != nil || d.Allowed || d.RetryAfter != 60 {
		t.Fatalf("bad denial%+v %v", d, err)
	}
	db.SetTime(time.Date(2026, 10, 29, 12, 1, 0, 0, time.UTC))
	d, err = second.Check(context.Background(), rule)
	if err != nil || !d.Allowed {
		t.Fatalf("window did not expire%+v %v", d, err)
	}
}
func TestBucketRefillAndMaximumWait(t *testing.T) {
	e, db := fixture(t)
	r := Bucket("normal", "user", 1200, 2)
	for i := 0; i < 2; i++ {
		d, err := e.Store.Check(context.Background(), r)
		if err != nil || !d.Allowed {
			t.Fatal(d, err)
		}
	}
	d, err := e.Store.Check(context.Background(), r)
	if err != nil || d.Allowed || d.RetryAfter != 1 {
		t.Fatal(d, err)
	}
	db.SetTime(time.Date(2026, 10, 29, 12, 0, 1, 0, time.UTC))
	d, err = e.Store.Check(context.Background(), r)
	if err != nil || !d.Allowed {
		t.Fatal(d, err)
	}
	a, b := Window("a", "user", 1, time.Minute), Window("b", "user", 1, time.Hour)
	if d, err = e.Store.Check(context.Background(), a, b); err != nil || !d.Allowed {
		t.Fatal(d, err)
	}
	d, err = e.Store.Check(context.Background(), a, b)
	if err != nil || d.RetryAfter != 3600 {
		t.Fatal(d, err)
	}
}
func TestRecipientQuotasAreAtomicAndDoNotConsumeOnSuppression(t *testing.T) {
	e, db := fixture(t)
	r := Window("recipient-hour", "private@example.test", 3, time.Hour)
	cool := Window("recipient-cooldown", r.Identity, 1, time.Minute)
	for i := 0; i < 3; i++ {
		db.SetTime(time.Date(2026, 10, 29, 12, i, 0, 0, time.UTC))
		d, err := e.Store.Check(context.Background(), r, cool)
		if err != nil || !d.Allowed {
			t.Fatal(d, err)
		}
		for j := 0; j < 5; j++ {
			d, err = e.Store.Check(context.Background(), r, cool)
			if err != nil || d.Allowed {
				t.Fatal(d, err)
			}
		}
	}
	db.SetTime(time.Date(2026, 10, 29, 12, 3, 0, 0, time.UTC))
	d, err := e.Store.Check(context.Background(), r, cool)
	if err != nil || d.Allowed || d.RetryAfter != 3420 {
		t.Fatal(d, err)
	}
	for _, key := range db.Keys() {
		if strings.Contains(key, "example") {
			t.Fatal("raw recipient stored")
		}
	}
}
func TestSuccessfulLoginReleasesOnlyItsOwnFailureReservation(t *testing.T) {
	e, _ := fixture(t)
	r := Window("login-pair", "ip|email", 5, 10*time.Minute)
	var last Decision
	for i := 0; i < 5; i++ {
		d, err := e.Store.Check(context.Background(), r)
		if err != nil || !d.Allowed {
			t.Fatal(d, err)
		}
		last = d
	}
	if err := e.Store.Release(context.Background(), r, last.Reservation); err != nil {
		t.Fatal(err)
	}
	d, err := e.Store.Check(context.Background(), r)
	if err != nil || !d.Allowed {
		t.Fatal(d, err)
	}
	d, err = e.Store.Check(context.Background(), r)
	if err != nil || d.Allowed {
		t.Fatal("success cleared other failures", d, err)
	}
}
func request(router http.Handler, path, ip, edge, asserted, service string) *httptest.ResponseRecorder {
	q := httptest.NewRequest("GET", "https://app.fabkli.ch"+path, nil)
	q.RemoteAddr = ip + ":1234"
	q.Header.Set("X-Forwarded-For", asserted)
	q.Header.Set("X-Fabkli-Client-IP", asserted)
	q.Header.Set("X-Fabkli-Edge-Key", edge)
	q.Header.Set("X-Fabkli-Service-Key", service)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, q)
	return w
}
func TestForgedForwardingAndServiceHeadersCannotSelectQuota(t *testing.T) {
	e, _ := fixture(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(e.Before())
	r.GET("/api/public", func(c *gin.Context) { c.String(200, c.GetString("ratelimit.ip")) })
	r.GET("/api/users/me", func(c *gin.Context) { c.String(200, strconvBool(c.GetBool("ratelimit.internal"))) })
	w := request(r, "/api/public", "192.0.2.1", "forged", "198.51.100.1", "")
	if w.Body.String() != "192.0.2.1" {
		t.Fatal(w.Body.String())
	}
	w = request(r, "/api/public", "192.0.2.1", e.EdgeKey, "198.51.100.1", "")
	if w.Body.String() != "198.51.100.1" {
		t.Fatal(w.Body.String())
	}
	w = request(r, "/api/users/me", "192.0.2.1", "", "", "forged")
	if w.Body.String() != "false" {
		t.Fatal("forged internal exception")
	}
	w = request(r, "/api/users/me", "192.0.2.1", "", "", e.ServiceKey)
	if w.Body.String() != "true" {
		t.Fatal("verified service rejected")
	}
}
func strconvBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
func TestOutageObserveAndRetryFeedback(t *testing.T) {
	e, db := fixture(t)
	gin.SetMode(gin.TestMode)
	rule := Bucket("tiny", "user", 1, 1)
	r := gin.New()
	r.GET("/normal", func(c *gin.Context) {
		if e.check(c, false, rule) {
			c.Status(204)
		}
	})
	r.GET("/sensitive", func(c *gin.Context) {
		if e.check(c, true, rule) {
			c.Status(204)
		}
	})
	if w := request(r, "/normal", "192.0.2.1", "", "", ""); w.Code != 204 {
		t.Fatal(w.Code)
	}
	e.Mode = "observe"
	if w := request(r, "/normal", "192.0.2.1", "", "", ""); w.Code != 204 || e.Observed.Load() != 1 {
		t.Fatal("observe-only blocked")
	}
	w := request(r, "/sensitive", "192.0.2.1", "", "", "")
	if w.Code != 429 || w.Header().Get("Retry-After") == "" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Header())
	}
	db.Close()
	w = request(r, "/sensitive", "192.0.2.1", "", "", "")
	if w.Code != 503 {
		t.Fatal("sensitive request failed open", w.Code)
	}
	w = request(r, "/normal", "192.0.2.1", "", "", "")
	if w.Code != 204 {
		t.Fatal("ordinary fallback failed", w.Code)
	}
	w = request(r, "/normal", "192.0.2.1", "", "", "")
	if w.Code != 429 {
		t.Fatal("ordinary fallback unbounded", w.Code)
	}
}
