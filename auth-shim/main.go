// auth-shim/main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

type server struct {
	jwksURL string
	kf      keyfunc.Keyfunc
	mu      sync.RWMutex
}

func newServer(jwksURL string) *server {
	return &server{jwksURL: jwksURL}
}

func (s *server) ensureJWKS() (keyfunc.Keyfunc, error) {
	s.mu.RLock()
	if s.kf != nil {
		kf := s.kf
		s.mu.RUnlock()
		return kf, nil
	}
	s.mu.RUnlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.kf != nil {
		return s.kf, nil
	}
	kf, err := keyfunc.NewDefaultCtx(context.Background(), []string{s.jwksURL})
	if err != nil {
		return nil, err
	}
	s.kf = kf
	return kf, nil
}

type streamClaims struct {
	Permissions []struct {
		Action string `json:"action"`
		Path   string `json:"path"`
	} `json:"mediamtx_permissions"`
	MunicipalityID string `json:"municipality_id"`
	CameraID       string `json:"camera_id"`
	jwt.RegisteredClaims
}

func (s *server) handleCheck(w http.ResponseWriter, r *http.Request) {
	tok := r.URL.Query().Get("token")
	if tok == "" {
		tok = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}
	if tok == "" {
		if c, err := r.Cookie("hub_view"); err == nil {
			tok = c.Value
		}
	}
	if tok == "" {
		http.Error(w, "no token", http.StatusUnauthorized)
		return
	}

	kf, err := s.ensureJWKS()
	if err != nil {
		http.Error(w, "jwks fetch failed", http.StatusServiceUnavailable)
		return
	}
	claims := &streamClaims{}
	parsed, err := jwt.ParseWithClaims(tok, claims, kf.Keyfunc, jwt.WithIssuer("mdrrmo-api"), jwt.WithValidMethods([]string{"ES256"}))
	if err != nil {
		log.Printf("jwt parse error: %v", err)
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	if !parsed.Valid {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	wantPath := r.URL.Query().Get("path")
	wantAction := r.URL.Query().Get("action")
	if len(claims.Permissions) == 0 || claims.Permissions[0].Path != wantPath || claims.Permissions[0].Action != wantAction {
		http.Error(w, "claim/path mismatch", http.StatusForbidden)
		return
	}

	// HLS browsers fetch index.m3u8 with ?jwt=, then request relative .ts URLs
	// that drop the query string. Mint an HttpOnly cookie scoped to the
	// stream-path so subsequent .ts segments authenticate via cookie. Caddy's
	// forward_auth `copy_response_headers Set-Cookie` (Path B Caddyfile)
	// propagates this back to the browser. Max-Age matches the 600s JWT TTL
	// ceiling enforced in mintStreamToken (super-admin-landing/src/lib/cctv/stream-token.ts).
	// Caddy forward_auth populates X-Forwarded-Uri with the original request URI.
	// Mint the cookie only on .m3u8 reads — .ts requests should already carry it.
	if wantAction == "read" && strings.HasSuffix(strings.SplitN(r.Header.Get("X-Forwarded-Uri"), "?", 2)[0], ".m3u8") {
		// Path-scoped cookie so cross-camera leakage is impossible at the browser layer.
		cookiePath := "/" + wantPath + "/"
		http.SetCookie(w, &http.Cookie{
			Name:     "hub_view",
			Value:    tok,
			Path:     cookiePath,
			MaxAge:   600,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
		})
	}

	// Emit one structured log line per accept; audit-batch cron tails the file.
	// Note: this is "every accept", not "first-of-bucket" — the audit-batch
	// endpoint spec (Phase 6, deferred) is responsible for dedupe.
	log.Printf(`{"ts":"%s","action":"%s","path":"%s","muni":"%s","cam":"%s","sub":"%s"}`,
		time.Now().UTC().Format(time.RFC3339), wantAction, wantPath, claims.MunicipalityID, claims.CameraID, claims.Subject)
	w.WriteHeader(http.StatusNoContent)
}

type mediaMtxAuthRequest struct {
	User     string `json:"user"`
	Password string `json:"password"`
	IP       string `json:"ip"`
	Action   string `json:"action"`   // publish | read
	Path     string `json:"path"`
	Query    string `json:"query"`    // raw query string ("jwt=eyJ...")
	Protocol string `json:"protocol"` // rtsps | rtsp | webrtc | hls | rtmp
}

// handleAuth implements the MediaMTX externalAuthenticationURL contract:
// MediaMTX POSTs a JSON body before allowing publish/read; we extract the
// JWT from the publish URL's ?jwt= query parameter, validate it against the
// JWKS, and enforce that mediamtx_permissions[0] matches the requested
// action+path. Same JWKS + claim shape as handleCheck (Caddy forward_auth);
// only the transport differs (POST JSON body vs GET query+headers+cookie).
func (s *server) handleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	var req mediaMtxAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// Pull JWT from the publish URL's query string.
	q, err := url.ParseQuery(req.Query)
	if err != nil {
		http.Error(w, "bad query", http.StatusBadRequest)
		return
	}
	tok := q.Get("jwt")
	if tok == "" {
		http.Error(w, "missing jwt", http.StatusUnauthorized)
		return
	}

	kf, err := s.ensureJWKS()
	if err != nil {
		http.Error(w, "jwks fetch failed", http.StatusServiceUnavailable)
		return
	}
	claims := &streamClaims{}
	parsed, err := jwt.ParseWithClaims(tok, claims, kf.Keyfunc,
		jwt.WithIssuer("mdrrmo-api"), jwt.WithValidMethods([]string{"ES256"}))
	if err != nil {
		log.Printf("jwt parse error (publish auth): %v", err)
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	if !parsed.Valid {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	if len(claims.Permissions) == 0 ||
		claims.Permissions[0].Action != req.Action ||
		claims.Permissions[0].Path != req.Path {
		http.Error(w, "claim/path/action mismatch", http.StatusForbidden)
		return
	}
	log.Printf(`{"ts":"%s","handler":"auth","action":"%s","path":"%s","ip":"%s","protocol":"%s","sub":"%s"}`,
		time.Now().UTC().Format(time.RFC3339), req.Action, req.Path, req.IP, req.Protocol, claims.Subject)
	w.WriteHeader(http.StatusOK)
}

func (s *server) primeJWKS(ctx context.Context) {
	// Bounded retry on startup so transient dashboard unreachability doesn't
	// leave the first viewer staring at a 503.
	backoff := 2 * time.Second
	for i := 1; i <= 5; i++ {
		if _, err := s.ensureJWKS(); err == nil {
			log.Printf("[startup] JWKS primed on attempt %d", i)
			return
		} else {
			log.Printf("[startup] JWKS prime attempt %d failed: %v (retry in %s)", i, err, backoff)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
			backoff *= 2
		}
	}
	log.Printf("[startup] JWKS prime exhausted; will retry lazily on first request")
}

func (s *server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	primed := s.kf != nil
	s.mu.RUnlock()
	if !primed {
		http.Error(w, "jwks not yet loaded", http.StatusServiceUnavailable)
		return
	}
	fmt.Fprintln(w, "ok")
}

func main() {
	// Self-healthcheck mode for Compose `healthcheck:` (distroless image has
	// no shell). Returns 0 if /healthz responds 200; 1 otherwise.
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		resp, err := http.Get("http://127.0.0.1:7000/healthz")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	jwksURL := os.Getenv("JWKS_URL")
	if jwksURL == "" {
		jwksURL = "https://" + os.Getenv("DASHBOARD_HOST") + "/.well-known/jwks.json"
	}
	s := newServer(jwksURL)

	// Prime JWKS in the background so /healthz can flip to 200 ASAP without
	// blocking the listener (Caddy/MediaMTX depend on the listener being up).
	go s.primeJWKS(context.Background())

	mux := http.NewServeMux()
	mux.HandleFunc("/check", s.handleCheck)
	mux.HandleFunc("/auth", s.handleAuth)
	mux.HandleFunc("/healthz", s.handleHealthz)
	addr := ":7000"
	log.Printf("auth-shim listening on %s, JWKS=%s", addr, jwksURL)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
