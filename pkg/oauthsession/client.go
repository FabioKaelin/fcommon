// Package oauthsession validates an F-OAuth session and safely carries renewal
// cookies back to the browser. It has no Gin dependency and also serves gateways.
package oauthsession

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var ErrUnauthenticated = errors.New("not authenticated")
var ErrUnavailable = errors.New("authentication service unavailable")

type User struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	Privileges int    `json:"privileges"`
}
type Result struct {
	User    User
	Cookies []*http.Cookie
}

type Client struct{ HTTP *http.Client }

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (c *Client) Validate(ctx context.Context, endpoint, token string) (Result, error) {
	var result Result
	if token == "" || len(token) > 8192 {
		return result, ErrUnauthenticated
	}
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return result, ErrUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return result, ErrUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if key := os.Getenv("RATELIMIT_SERVICE_KEY"); key != "" {
		req.Header.Set("X-Fabkli-Service-Key", key)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return result, ErrUnavailable
	}
	defer res.Body.Close()
	if res.StatusCode == 401 || res.StatusCode == 403 {
		return result, ErrUnauthenticated
	}
	if res.StatusCode != 200 {
		return result, ErrUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 65537))
	if err != nil || len(body) > 65536 || json.Unmarshal(body, &result.User) != nil || result.User.ID == "" || result.User.ID == "00000000-0000-0000-0000-000000000000" {
		return Result{}, ErrUnavailable
	}
	// Only F-OAuth's shared-domain auth cookie may escape this trusted response.
	// Never relay arbitrary cookies or accept weakened cookie attributes.
	for _, cookie := range res.Cookies() {
		if cookie.Name == "token" && strings.TrimPrefix(cookie.Domain, ".") == "fabkli.ch" && cookie.Path == "/" && cookie.Secure && cookie.HttpOnly && cookie.SameSite == http.SameSiteLaxMode && cookie.MaxAge > 0 && cookie.MaxAge <= 30*24*60*60 && cookie.Value == token {
			result.Cookies = append(result.Cookies, cookie)
		}
	}
	return result, nil
}
func (r Result) ForwardCookies(w http.ResponseWriter) {
	for _, cookie := range r.Cookies {
		http.SetCookie(w, cookie)
	}
	w.Header().Set("Cache-Control", "no-store")
}
