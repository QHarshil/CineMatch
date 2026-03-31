package middleware

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const contextKeyUserID contextKey = "userID"

// jwksCache holds the parsed JWKS keys fetched from Supabase.
// Refreshed at most once per hour to avoid hitting the JWKS endpoint on every request.
type jwksCache struct {
	mu      sync.RWMutex
	keys    map[string]*ecdsa.PublicKey
	fetched time.Time
	jwksURL string
}

var globalJWKS = &jwksCache{}

// RequireAuth returns a middleware that verifies Supabase JWTs and injects the user UUID
// into the request context. Supports both HS256 (legacy) and ES256 (current Supabase signing).
func RequireAuth(jwtSecret string) func(http.Handler) http.Handler {
	supabaseURL := os.Getenv("SUPABASE_URL")
	if supabaseURL != "" {
		globalJWKS.jwksURL = strings.TrimRight(supabaseURL, "/") + "/auth/v1/.well-known/jwks.json"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := extractBearerToken(r)
			if !ok {
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			userID, err := verifySupabaseJWT(token, jwtSecret)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), contextKeyUserID, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext extracts the authenticated user UUID injected by RequireAuth.
// Returns ("", false) if the middleware was not applied to this request.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(contextKeyUserID).(string)
	return id, ok && id != ""
}

// WithUserID returns a copy of ctx with the given userID injected.
// Intended for handler tests that need to simulate an authenticated request
// without running JWT verification.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, contextKeyUserID, userID)
}

// extractBearerToken parses "Bearer <token>" from the Authorization header.
// Falls back to X-Authorization because Railway's CDN edge (Fastly) strips
// the standard Authorization header on proxied requests.
func extractBearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if header == "" {
		header = r.Header.Get("X-Authorization")
	}
	if !strings.HasPrefix(header, "Bearer ") {
		return "", false
	}
	token := strings.TrimPrefix(header, "Bearer ")
	return token, token != ""
}

// verifySupabaseJWT validates a Supabase JWT and returns the subject (user UUID).
// Tries ES256 verification via JWKS first (current Supabase signing), then falls
// back to HS256 with the project's JWT secret (legacy signing).
func verifySupabaseJWT(tokenString, hmacSecret string) (string, error) {
	// Try ES256 via JWKS (current Supabase default)
	if globalJWKS.jwksURL != "" {
		userID, err := verifyES256(tokenString)
		if err == nil {
			return userID, nil
		}
	}

	// Fall back to HS256 (legacy Supabase projects)
	return verifyHS256(tokenString, hmacSecret)
}

func verifyES256(tokenString string) (string, error) {
	parsed, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, fmt.Errorf("missing kid in token header")
		}
		key, err := globalJWKS.getKey(kid)
		if err != nil {
			return nil, err
		}
		return key, nil
	}, jwt.WithValidMethods([]string{"ES256"}))
	if err != nil {
		return "", err
	}
	return extractSubject(parsed)
}

func verifyHS256(tokenString, secret string) (string, error) {
	parsed, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return "", err
	}
	return extractSubject(parsed)
}

func extractSubject(parsed *jwt.Token) (string, error) {
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return "", jwt.ErrTokenInvalidClaims
	}
	sub, err := claims.GetSubject()
	if err != nil || sub == "" {
		return "", jwt.ErrTokenInvalidClaims
	}
	return sub, nil
}

// getKey returns the ECDSA public key for the given kid, fetching JWKS if stale.
func (j *jwksCache) getKey(kid string) (*ecdsa.PublicKey, error) {
	j.mu.RLock()
	if key, ok := j.keys[kid]; ok && time.Since(j.fetched) < 1*time.Hour {
		j.mu.RUnlock()
		return key, nil
	}
	j.mu.RUnlock()

	// Fetch fresh JWKS
	if err := j.refresh(); err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}

	j.mu.RLock()
	defer j.mu.RUnlock()
	key, ok := j.keys[kid]
	if !ok {
		return nil, fmt.Errorf("key %s not found in JWKS", kid)
	}
	return key, nil
}

func (j *jwksCache) refresh() error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(j.jwksURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return err
	}

	keys := make(map[string]*ecdsa.PublicKey)
	for _, raw := range jwks.Keys {
		var k jwkKey
		if err := json.Unmarshal(raw, &k); err != nil {
			continue
		}
		if k.Kty != "EC" || k.Crv != "P-256" || k.Kid == "" {
			continue
		}
		pub, err := k.toPublicKey()
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}

	j.mu.Lock()
	j.keys = keys
	j.fetched = time.Now()
	j.mu.Unlock()
	return nil
}

// jwkKey represents an EC JWK key from Supabase's JWKS endpoint.
type jwkKey struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
	Kid string `json:"kid"`
	// Some JWKS responses include x5c (X.509 certificate chain)
	X5C []string `json:"x5c"`
}

func (k *jwkKey) toPublicKey() (*ecdsa.PublicKey, error) {
	// Prefer x5c certificate chain if available
	if len(k.X5C) > 0 {
		certDER, err := base64.StdEncoding.DecodeString(k.X5C[0])
		if err == nil {
			cert, err := x509.ParseCertificate(certDER)
			if err == nil {
				if pub, ok := cert.PublicKey.(*ecdsa.PublicKey); ok {
					return pub, nil
				}
			}
		}
	}

	// Fall back to raw x/y coordinates
	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, fmt.Errorf("invalid x coordinate: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, fmt.Errorf("invalid y coordinate: %w", err)
	}

	return &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}, nil
}
