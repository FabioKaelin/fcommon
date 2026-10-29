package oauthsession

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRenewalAndFailureBoundaries(t *testing.T) {
	for _, test := range []struct {
		name, body, cookie string
		status             int
		err                error
		count              int
	}{
		{"renewal", `{"id":"owner"}`, "token=opaque; Domain=fabkli.ch; Path=/; Max-Age=2592000; Secure; HttpOnly; SameSite=Lax", 200, nil, 1},
		{"weak cookie", `{"id":"owner"}`, "token=opaque; Domain=fabkli.ch; Path=/; Max-Age=2592000", 200, nil, 0},
		{"different credential", `{"id":"owner"}`, "token=attacker; Domain=fabkli.ch; Path=/; Max-Age=2592000; Secure; HttpOnly; SameSite=Lax", 200, nil, 0},
		{"foreign domain", `{"id":"owner"}`, "token=opaque; Domain=evil.com; Path=/; Max-Age=2592000; Secure; HttpOnly; SameSite=Lax", 200, nil, 0},
		{"invalid credential", `{"id":"owner"}`, "", 401, ErrUnauthenticated, 0},
		{"revoked credential", `{"id":"owner"}`, "", 403, ErrUnauthenticated, 0},
		{"outage", `{"id":"owner"}`, "", 500, ErrUnavailable, 0},
		{"empty identity", `{}`, "", 200, ErrUnavailable, 0},
		{"missing database user", `{"id":"00000000-0000-0000-0000-000000000000"}`, "", 200, ErrUnavailable, 0},
		{"malformed response", `no-json`, "", 200, ErrUnavailable, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer opaque" {
					t.Error("credential not supplied")
				}
				if test.cookie != "" {
					w.Header().Add("Set-Cookie", test.cookie)
				}
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			result, err := NewClient().Validate(context.Background(), server.URL, "opaque")
			if !errors.Is(err, test.err) {
				t.Fatalf("got %v want %v", err, test.err)
			}
			if len(result.Cookies) != test.count {
				t.Fatalf("cookie count %d", len(result.Cookies))
			}
			w := httptest.NewRecorder()
			result.ForwardCookies(w)
			if len(w.Result().Cookies()) != test.count {
				t.Error("browser renewal not forwarded")
			}
		})
	}
}
