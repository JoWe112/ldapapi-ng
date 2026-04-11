package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/JoWe112/ldapapi-ng/internal/config"
	ldapclient "github.com/JoWe112/ldapapi-ng/internal/ldap"
)

// fakeLDAP is a hand-rolled test double for ldapclient.Client.
type fakeLDAP struct {
	authErr    error
	lookupAttr map[string][]string
	lookupErr  error

	lastAuthUser string
	lastAuthPass string
	lastLookup   string
}

func (f *fakeLDAP) Authenticate(_ context.Context, user, pass string) error {
	f.lastAuthUser = user
	f.lastAuthPass = pass
	return f.authErr
}

func (f *fakeLDAP) LookupUser(_ context.Context, uid string) (map[string][]string, error) {
	f.lastLookup = uid
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	// Return a copy so the handler can mutate it safely.
	out := make(map[string][]string, len(f.lookupAttr))
	for k, v := range f.lookupAttr {
		out[k] = v
	}
	return out, nil
}

func newTestRouter(t *testing.T, ldap ldapclient.Client, mode config.AuthMode) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := &Router{
		Config: &config.Config{AuthMode: mode, DevMode: true},
		LDAP:   ldap,
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	return r.Build()
}

func basicHeader(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

func TestHealth(t *testing.T) {
	h := newTestRouter(t, &fakeLDAP{}, config.AuthModeGateway)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("expected status=ok, got %q", body.Status)
	}
}

func TestAuth_Success(t *testing.T) {
	fake := &fakeLDAP{}
	h := newTestRouter(t, fake, config.AuthModeGateway)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth", nil)
	req.Header.Set("Authorization", basicHeader("jdoe", "secret"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	var body AuthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Authenticated || body.Username != "jdoe" {
		t.Errorf("unexpected body: %+v", body)
	}
	if fake.lastAuthUser != "jdoe" || fake.lastAuthPass != "secret" {
		t.Errorf("credentials not forwarded: user=%q pass=%q", fake.lastAuthUser, fake.lastAuthPass)
	}
}

func TestAuth_MissingHeader(t *testing.T) {
	h := newTestRouter(t, &fakeLDAP{}, config.AuthModeGateway)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("WWW-Authenticate"), "Basic") {
		t.Errorf("missing WWW-Authenticate header")
	}
}

func TestAuth_InvalidCredentials(t *testing.T) {
	h := newTestRouter(t, &fakeLDAP{authErr: ldapclient.ErrInvalidCredentials}, config.AuthModeGateway)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth", nil)
	req.Header.Set("Authorization", basicHeader("jdoe", "wrong"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	var body ErrorBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "INVALID_CREDENTIALS" {
		t.Errorf("expected INVALID_CREDENTIALS, got %q", body.Error.Code)
	}
}

func TestUser_Success(t *testing.T) {
	fake := &fakeLDAP{
		lookupAttr: map[string][]string{
			"dn":   {"uid=jdoe,ou=people,dc=example,dc=org"},
			"cn":   {"John Doe"},
			"mail": {"jdoe@example.org"},
		},
	}
	h := newTestRouter(t, fake, config.AuthModeGateway)

	req := httptest.NewRequest(http.MethodGet, "/v1/user/jdoe", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	var body UserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.UID != "jdoe" {
		t.Errorf("uid mismatch: %q", body.UID)
	}
	if body.DN != "uid=jdoe,ou=people,dc=example,dc=org" {
		t.Errorf("dn mismatch: %q", body.DN)
	}
	if _, hasDN := body.Attributes["dn"]; hasDN {
		t.Errorf("dn should be stripped from Attributes")
	}
	if got := body.Attributes["mail"]; len(got) != 1 || got[0] != "jdoe@example.org" {
		t.Errorf("mail mismatch: %v", got)
	}
}

func TestUser_NotFound(t *testing.T) {
	h := newTestRouter(t, &fakeLDAP{lookupErr: ldapclient.ErrUserNotFound}, config.AuthModeGateway)
	req := httptest.NewRequest(http.MethodGet, "/v1/user/ghost", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	var body ErrorBody
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Error.Code != "USER_NOT_FOUND" {
		t.Errorf("expected USER_NOT_FOUND, got %q", body.Error.Code)
	}
}

func TestStandaloneMode_RequiresBasicAuth(t *testing.T) {
	// In standalone mode the middleware must block unauthenticated requests
	// even to /v1/user/:uid.
	h := newTestRouter(t, &fakeLDAP{authErr: ldapclient.ErrInvalidCredentials}, config.AuthModeStandalone)
	req := httptest.NewRequest(http.MethodGet, "/v1/user/jdoe", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 from middleware, got %d", w.Code)
	}
}

func TestParseBasicAuth(t *testing.T) {
	cases := []struct {
		name   string
		header string
		user   string
		pass   string
		ok     bool
	}{
		{"valid", basicHeader("alice", "pw"), "alice", "pw", true},
		{"missing prefix", "Bearer xyz", "", "", false},
		{"bad base64", "Basic !!!", "", "", false},
		{"no colon", "Basic " + base64.StdEncoding.EncodeToString([]byte("nocolon")), "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, p, ok := parseBasicAuth(tc.header)
			if ok != tc.ok || u != tc.user || p != tc.pass {
				t.Errorf("got (%q,%q,%v), want (%q,%q,%v)", u, p, ok, tc.user, tc.pass, tc.ok)
			}
		})
	}
}
