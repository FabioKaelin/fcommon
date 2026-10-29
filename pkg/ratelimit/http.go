package ratelimit

import (
	"context"
	"crypto/subtle"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

var Default *Engine

// Report emits only aggregate counters, with no IP/user/email/token labels.
// Callers cancel the context on shutdown; repeated totals are not event counts.
func (e *Engine) Report(ctx context.Context) {
	if e == nil {
		return
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Printf("rate_limit service=%s mode=%s allowed_total=%d limited_total=%d observed_total=%d store_errors_total=%d", e.Store.Namespace, e.Mode, e.Allowed.Load(), e.Limited.Load(), e.Observed.Load(), e.StoreErrors.Load())
		}
	}
}

type Engine struct {
	Store                                      *Store
	Mode, EdgeKey, ServiceKey                  string
	AuthenticatedPerMinute, AuthenticatedBurst int
	local                                      fallback
	Allowed, Limited, Observed, StoreErrors    atomic.Uint64
	lastAlert                                  atomic.Int64
}

// FromEnv is deliberately disabled unless configured. Deployments must supply
// independent random secrets, not public headers or a private-address exception.
func FromEnv(namespace string) (*Engine, error) {
	mode := os.Getenv("RATELIMIT_MODE")
	if mode == "" || mode == "disabled" {
		return nil, nil
	}
	if mode != "observe" && mode != "enforce" {
		return nil, errors.New("invalid RATELIMIT_MODE")
	}
	key, edge, service := os.Getenv("RATELIMIT_KEY"), os.Getenv("RATELIMIT_EDGE_KEY"), os.Getenv("RATELIMIT_SERVICE_KEY")
	if len(key) < 32 || len(edge) < 32 || len(service) < 32 {
		return nil, errors.New("rate limiting requires independent secret keys of at least32bytes")
	}
	address, password := os.Getenv("RATELIMIT_REDIS_ADDR"), os.Getenv("RATELIMIT_REDIS_PASSWORD")
	if address == "" || password == "" {
		return nil, errors.New("rate limiting requires authenticated Redis")
	}
	client := redis.NewClient(&redis.Options{Addr: address, Password: password, PoolSize: 4, MinIdleConns: 0, MaxRetries: -1, DialTimeout: 200 * time.Millisecond, ReadTimeout: 200 * time.Millisecond, WriteTimeout: 200 * time.Millisecond, PoolTimeout: 200 * time.Millisecond, ContextTimeoutEnabled: true})
	rate, burst := 1200, 120
	for name, target := range map[string]*int{"RATELIMIT_AUTH_PER_MINUTE": &rate, "RATELIMIT_AUTH_BURST": &burst} {
		if value := os.Getenv(name); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > 100000 {
				return nil, errors.New("invalid authenticated rate policy")
			}
			*target = n
		}
	}
	return &Engine{Store: &Store{Client: client, Namespace: namespace, Secret: []byte(key)}, Mode: mode, EdgeKey: edge, ServiceKey: service, AuthenticatedPerMinute: rate, AuthenticatedBurst: burst, local: fallback{now: time.Now}}, nil
}
func match(got, want string) bool {
	return len(want) >= 32 && len(got) == len(want) && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
func (e *Engine) internal(c *gin.Context) bool {
	return c.Request.Method == http.MethodGet && (c.Request.URL.Path == "/api/users/me" || c.Request.URL.Path == "/api/sessions/check") && match(c.GetHeader("X-Fabkli-Service-Key"), e.ServiceKey)
}
func Sensitive(c *gin.Context) bool {
	p := c.Request.URL.Path
	return strings.HasPrefix(p, "/api/auth/login") || p == "/api/auth/register" || strings.HasPrefix(p, "/api/password/") || strings.HasPrefix(p, "/api/sessions/oauth/")
}
func (e *Engine) respond(c *gin.Context, status, seconds int) {
	if seconds < 1 {
		seconds = 1
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Retry-After", strconv.Itoa(seconds))
	message := "Zu viele Anfragen. Bitte in " + strconv.Itoa(seconds) + " Sekunden erneut versuchen."
	code := "rate_limited"
	if status == 503 {
		code = "temporarily_unavailable"
		message = "Der Dienst ist vorübergehend nicht verfügbar. Bitte in " + strconv.Itoa(seconds) + " Sekunden erneut versuchen."
	}
	if strings.Contains(c.GetHeader("Accept"), "text/html") && strings.HasPrefix(c.Request.URL.Path, "/api/sessions/oauth/") {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(status, "<!doctype html><html lang=de><meta name=viewport content='width=device-width,initial-scale=1'><title>Anmeldung kurz pausiert</title><main><h1>Anmeldung kurz pausiert</h1><p role=alert>%s</p><p>Ihre Eingaben bleiben erhalten. Starten Sie die Anmeldung danach neu.</p><a href='/login'>Zur Anmeldung</a></main></html>", message)
		c.Abort()
		return
	}
	c.AbortWithStatusJSON(status, gin.H{"status": "fail", "code": code, "message": message, "retry_after": seconds})
}
func (e *Engine) check(c *gin.Context, sensitive bool, rules ...Rule) bool {
	if e == nil {
		return true
	}
	d, err := e.Store.Check(c.Request.Context(), rules...)
	if err != nil {
		e.StoreErrors.Add(1)
		now := time.Now().Unix()
		last := e.lastAlert.Load()
		if now-last >= 60 && e.lastAlert.CompareAndSwap(last, now) {
			log.Printf("rate_limit store_unavailable service=%s; ordinary protection is per-process", e.Store.Namespace)
		}
		if sensitive {
			e.respond(c, 503, 30)
			return false
		}
		d = Decision{Allowed: true}
		for _, r := range rules {
			if r.Burst == 0 {
				continue
			}
			one := e.local.check(e.Store.key(r), r)
			if !one.Allowed {
				d.Allowed = false
				if one.RetryAfter > d.RetryAfter {
					d.RetryAfter = one.RetryAfter
				}
			}
		}
	}
	if !d.Allowed {
		if !sensitive && e.Mode == "observe" && err == nil {
			e.Observed.Add(1)
			return true
		}
		e.Limited.Add(1)
		e.respond(c, 429, d.RetryAfter)
		return false
	}
	e.Allowed.Add(1)
	return true
}
func public(c *gin.Context) bool {
	p := c.FullPath()
	return p == "" || strings.HasPrefix(p, "/api/swagger") || strings.HasPrefix(p, "/api/auth/") || strings.HasPrefix(p, "/api/sessions/oauth/") || p == "/api/password/forgot" || p == "/api/password/reset/:secret" || p == "/api/users/:userid/image"
}

// Before runs before authentication/password hashing. Only exact inexpensive
// health routes are exempt; internal session traffic requires a real secret.
func (e *Engine) Before() gin.HandlerFunc {
	return func(c *gin.Context) {
		if e == nil {
			c.Next()
			return
		}
		p := c.Request.URL.Path
		if p == "/internal/health/live" || p == "/internal/health/ready" || p == "/api/ping" || p == "/api/version" || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		if e.internal(c) {
			c.Set("ratelimit.internal", true)
			if e.check(c, false, Bucket("service-validation", "trusted-services", 6000, 600)) {
				c.Next()
			}
			return
		}
		// Only the authenticated edge may assert a forwarded address. Direct callers
		// use their TCP peer and cannot choose an IP through arbitrary headers.
		peer, _, err := net.SplitHostPort(c.Request.RemoteAddr)
		if err != nil {
			peer = c.Request.RemoteAddr
		}
		ip := net.ParseIP(peer)
		if match(c.GetHeader("X-Fabkli-Edge-Key"), e.EdgeKey) {
			ip = net.ParseIP(c.GetHeader("X-Fabkli-Client-IP"))
		}
		if ip == nil {
			c.AbortWithStatusJSON(400, gin.H{"code": "invalid_client_address"})
			return
		}
		c.Set("ratelimit.ip", ip.String())
		if !e.check(c, Sensitive(c), Bucket("flood", ip.String(), 1200, 200)) {
			return
		}
		if public(c) && !e.check(c, Sensitive(c), Bucket("anonymous", ip.String(), 120, 30)) {
			return
		}
		var r *Rule
		makeRule := func(name string, n int, d time.Duration) { q := Window(name, ip.String(), n, d); r = &q }
		switch {
		case c.Request.Method == "POST" && p == "/api/auth/login":
			makeRule("login-ip", 30, 10*time.Minute)
		case c.Request.Method == "POST" && p == "/api/auth/register":
			makeRule("register-ip", 5, time.Hour)
		case c.Request.Method == "POST" && p == "/api/password/forgot":
			makeRule("forgot-ip", 5, 15*time.Minute)
		case strings.HasPrefix(p, "/api/password/"):
			makeRule("password-ip", 10, 15*time.Minute)
		case strings.HasPrefix(p, "/api/sessions/oauth/start/"):
			makeRule("oauth-start", 30, 10*time.Minute)
		case strings.HasPrefix(p, "/api/sessions/oauth/"):
			makeRule("oauth-callback", 30, 10*time.Minute)
		}
		if r != nil && !e.check(c, true, *r) {
			return
		}
		c.Next()
	}
}

// User must only be called after the session identity has been verified, before
// application work. Submitted IDs, headers or raw tokens never reach this API.
func User(c *gin.Context, id string) bool {
	e := Default
	if e == nil || c.GetBool("ratelimit.internal") {
		return true
	}
	if !e.check(c, false, Bucket("authenticated", id, e.AuthenticatedPerMinute, e.AuthenticatedBurst)) {
		return false
	}
	if strings.HasPrefix(c.Request.URL.Path, "/api/password/") {
		return e.check(c, true, Window("password-user", id, 5, 15*time.Minute))
	}
	return true
}

// ReserveLogin bounds concurrent expensive password checks for each IP/email
// pair. A successful check releases just its reservation, preserving failures.
func ReserveLogin(c *gin.Context, email string) (func(bool), bool) {
	e := Default
	if e == nil {
		return func(bool) {}, true
	}
	if len(email) > 320 {
		c.AbortWithStatus(400)
		return nil, false
	}
	r := Window("login-pair", c.GetString("ratelimit.ip")+"|"+strings.ToLower(strings.TrimSpace(email)), 5, 10*time.Minute)
	d, err := e.Store.Check(c.Request.Context(), r)
	if err != nil {
		e.StoreErrors.Add(1)
		e.respond(c, 503, 30)
		return nil, false
	}
	if !d.Allowed {
		e.respond(c, 429, d.RetryAfter)
		return nil, false
	}
	return func(success bool) {
		if success {
			if err := e.Store.Release(c.Request.Context(), r, d.Reservation); err != nil {
				e.StoreErrors.Add(1)
			}
		}
	}, true
}

// ReserveRecipient is called before any lookup, notification or email, including
// unknown recipients and external-provider accounts. Suppression stays generic.
func ReserveRecipient(c *gin.Context, email string) bool {
	e := Default
	if e == nil {
		return true
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) > 320 {
		return false
	}
	d, err := e.Store.Check(c.Request.Context(), Window("recipient-hour", email, 3, time.Hour), Window("recipient-cooldown", email, 1, time.Minute))
	if err != nil {
		e.StoreErrors.Add(1)
		e.respond(c, 503, 30)
		return false
	}
	return d.Allowed
}
