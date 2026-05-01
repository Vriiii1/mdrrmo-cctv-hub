// auth-shim/main_test.go
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/golang-jwt/jwt/v5"
)

// setupTestJWKS generates a fresh ES256 keypair and serves the public half
// from an in-memory JWKS endpoint. Caller must defer srv.Close().
func setupTestJWKS(t *testing.T) (*ecdsa.PrivateKey, *httptest.Server) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	jwk, err := jwkset.NewJWKFromKey(priv.Public(), jwkset.JWKOptions{
		Metadata: jwkset.JWKMetadataOptions{KID: "test-kid", ALG: jwkset.AlgES256, USE: jwkset.UseSig},
	})
	if err != nil {
		t.Fatal(err)
	}
	js := jwkset.NewMemoryStorage()
	if err := js.KeyWrite(context.Background(), jwk); err != nil {
		t.Fatal(err)
	}
	raw, err := js.JSONPublic(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	return priv, srv
}

// mintTestToken signs a token with the provided ES256 private key and "test-kid" header.
func mintTestToken(t *testing.T, priv *ecdsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tok.Header["kid"] = "test-kid"
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

// baseClaims returns the iss/sub/iat/exp boilerplate every test needs;
// individual tests overlay action/path/permissions on top.
func baseClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"iss": "mdrrmo-api",
		"sub": "u1",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(10 * time.Minute).Unix(),
	}
}

// --- handleCheck (HLS/WebRTC forward_auth path) -----------------------------

func TestValidateRejectsMissingToken(t *testing.T) {
	s := newServer("https://example.com/.well-known/jwks.json")
	req := httptest.NewRequest("GET", "/check?path=muni-x/cam-y&action=read", nil)
	w := httptest.NewRecorder()
	s.handleCheck(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestValidateAcceptsCorrectToken(t *testing.T) {
	priv, srv := setupTestJWKS(t)
	defer srv.Close()

	claims := baseClaims()
	claims["mediamtx_permissions"] = []map[string]string{{"action": "read", "path": "muni-x/cam-y"}}
	claims["municipality_id"] = "m1"
	claims["camera_id"] = "c1"
	signed := mintTestToken(t, priv, claims)

	s := newServer(srv.URL)
	req := httptest.NewRequest("GET", "/check?path=muni-x/cam-y&action=read&token="+signed, nil)
	w := httptest.NewRecorder()
	s.handleCheck(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestValidateRejectsPathMismatch(t *testing.T) {
	priv, srv := setupTestJWKS(t)
	defer srv.Close()

	// Token claims path muni-a/cam-1, but request asks for muni-z/cam-99 → expect 403.
	claims := baseClaims()
	claims["mediamtx_permissions"] = []map[string]string{{"action": "read", "path": "muni-a/cam-1"}}
	signed := mintTestToken(t, priv, claims)

	s := newServer(srv.URL)
	req := httptest.NewRequest("GET", "/check?path=muni-z/cam-99&action=read&token="+signed, nil)
	w := httptest.NewRecorder()
	s.handleCheck(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestValidateRejectsEmptyPermissions(t *testing.T) {
	priv, srv := setupTestJWKS(t)
	defer srv.Close()

	// Token with EMPTY mediamtx_permissions array → expect 403 (claim/path mismatch).
	claims := baseClaims()
	claims["mediamtx_permissions"] = []map[string]string{}
	signed := mintTestToken(t, priv, claims)

	s := newServer(srv.URL)
	req := httptest.NewRequest("GET", "/check?path=muni-x/cam-y&action=read&token="+signed, nil)
	w := httptest.NewRecorder()
	s.handleCheck(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", w.Code, w.Body.String())
	}
}

// --- handleAuth (MediaMTX externalAuthenticationURL publish-gate path) ------

// MediaMTX POSTs JSON shaped per its externalAuthenticationURL contract:
//   {"user":"","password":"","ip":"...","action":"publish","path":"muni-x/cam-y",
//    "query":"jwt=eyJ...","protocol":"rtsps"}
// We accept iff token validates AND mediamtx_permissions[0] matches (action,path).

func TestAuthHandler_AcceptsPublishWithCorrectToken(t *testing.T) {
	priv, srv := setupTestJWKS(t)
	defer srv.Close()

	claims := baseClaims()
	claims["mediamtx_permissions"] = []map[string]string{{"action": "publish", "path": "muni-x/cam-y"}}
	claims["municipality_id"] = "muni-x"
	claims["camera_id"] = "cam-y"
	signed := mintTestToken(t, priv, claims)

	body := `{"user":"","password":"","ip":"10.0.0.1","action":"publish","path":"muni-x/cam-y","query":"jwt=` + signed + `","protocol":"rtsps"}`
	req := httptest.NewRequest("POST", "/auth", strings.NewReader(body))
	w := httptest.NewRecorder()
	s := newServer(srv.URL)
	s.handleAuth(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAuthHandler_RejectsPublishWithReadToken(t *testing.T) {
	priv, srv := setupTestJWKS(t)
	defer srv.Close()

	// Token says action=read but request asks for action=publish → expect 403.
	claims := baseClaims()
	claims["mediamtx_permissions"] = []map[string]string{{"action": "read", "path": "muni-x/cam-y"}}
	signed := mintTestToken(t, priv, claims)

	body := `{"user":"","password":"","ip":"10.0.0.1","action":"publish","path":"muni-x/cam-y","query":"jwt=` + signed + `","protocol":"rtsps"}`
	req := httptest.NewRequest("POST", "/auth", strings.NewReader(body))
	w := httptest.NewRecorder()
	s := newServer(srv.URL)
	s.handleAuth(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAuthHandler_RejectsPublishWithWrongTenantPath(t *testing.T) {
	priv, srv := setupTestJWKS(t)
	defer srv.Close()

	// Token claims publish on muni-other/cam-1 but request asks muni-x/cam-y → expect 403.
	// This is the cross-tenant-publish guard: an agent must not be able to
	// re-target a different municipality's path.
	claims := baseClaims()
	claims["mediamtx_permissions"] = []map[string]string{{"action": "publish", "path": "muni-other/cam-1"}}
	signed := mintTestToken(t, priv, claims)

	body := `{"user":"","password":"","ip":"10.0.0.1","action":"publish","path":"muni-x/cam-y","query":"jwt=` + signed + `","protocol":"rtsps"}`
	req := httptest.NewRequest("POST", "/auth", strings.NewReader(body))
	w := httptest.NewRecorder()
	s := newServer(srv.URL)
	s.handleAuth(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAuthHandler_RejectsPublishWithoutJWTQuery(t *testing.T) {
	_, srv := setupTestJWKS(t)
	defer srv.Close()

	// Empty query string is valid input to url.ParseQuery, but jwt= is missing
	// → expect 401 (missing jwt), NOT 400 (bad query).
	body := `{"user":"","password":"","ip":"10.0.0.1","action":"publish","path":"muni-x/cam-y","query":"","protocol":"rtsps"}`
	req := httptest.NewRequest("POST", "/auth", strings.NewReader(body))
	w := httptest.NewRecorder()
	s := newServer(srv.URL)
	s.handleAuth(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", w.Code, w.Body.String())
	}
}
