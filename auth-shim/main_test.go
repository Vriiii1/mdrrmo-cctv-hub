// auth-shim/main_test.go
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/golang-jwt/jwt/v5"
)

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
	defer srv.Close()

	claims := jwt.MapClaims{
		"mediamtx_permissions": []map[string]string{{"action": "read", "path": "muni-x/cam-y"}},
		"municipality_id":      "m1",
		"camera_id":            "c1",
		"iss":                  "mdrrmo-api",
		"sub":                  "u1",
		"iat":                  time.Now().Unix(),
		"exp":                  time.Now().Add(10 * time.Minute).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tok.Header["kid"] = "test-kid"
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}

	s := newServer(srv.URL)
	req := httptest.NewRequest("GET", "/check?path=muni-x/cam-y&action=read&token="+signed, nil)
	w := httptest.NewRecorder()
	s.handleCheck(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestValidateRejectsPathMismatch(t *testing.T) {
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
	defer srv.Close()

	// Token claims path muni-a/cam-1, but request asks for muni-z/cam-99 → expect 403.
	claims := jwt.MapClaims{
		"mediamtx_permissions": []map[string]string{{"action": "read", "path": "muni-a/cam-1"}},
		"iss":                  "mdrrmo-api",
		"sub":                  "u1",
		"iat":                  time.Now().Unix(),
		"exp":                  time.Now().Add(10 * time.Minute).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tok.Header["kid"] = "test-kid"
	signed, _ := tok.SignedString(priv)

	s := newServer(srv.URL)
	req := httptest.NewRequest("GET", "/check?path=muni-z/cam-99&action=read&token="+signed, nil)
	w := httptest.NewRecorder()
	s.handleCheck(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestValidateRejectsEmptyPermissions(t *testing.T) {
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
	defer srv.Close()

	// Token with EMPTY mediamtx_permissions array → expect 403 (claim/path mismatch).
	claims := jwt.MapClaims{
		"mediamtx_permissions": []map[string]string{},
		"iss":                  "mdrrmo-api",
		"sub":                  "u1",
		"iat":                  time.Now().Unix(),
		"exp":                  time.Now().Add(10 * time.Minute).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tok.Header["kid"] = "test-kid"
	signed, _ := tok.SignedString(priv)

	s := newServer(srv.URL)
	req := httptest.NewRequest("GET", "/check?path=muni-x/cam-y&action=read&token="+signed, nil)
	w := httptest.NewRecorder()
	s.handleCheck(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", w.Code, w.Body.String())
	}
}
