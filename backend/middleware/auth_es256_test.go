package middleware_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/harshilc/cinematch-backend/middleware"
)

// jwksServer serves one P-256 key the way Supabase's JWKS endpoint does and
// counts how often it is fetched.
func jwksServer(t *testing.T, kid string, pub *ecdsa.PublicKey) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var fetches atomic.Int32
	x, y := pub.X.FillBytes(make([]byte, 32)), pub.Y.FillBytes(make([]byte, 32))
	body, _ := json.Marshal(map[string]any{"keys": []map[string]string{{
		"kty": "EC", "crv": "P-256", "kid": kid,
		"x": base64.RawURLEncoding.EncodeToString(x), "y": base64.RawURLEncoding.EncodeToString(y),
	}}})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetches.Add(1)
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &fetches
}

func signES256(t *testing.T, key *ecdsa.PrivateKey, kid, sub string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"sub": sub, "aud": "authenticated", "exp": time.Now().Add(time.Hour).Unix(),
	})
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func TestRequireAuthVerifiesES256AgainstJWKS(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	srv, fetches := jwksServer(t, "k1", &key.PublicKey)
	const userID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{name: "token signed with the published key", token: signES256(t, key, "k1", userID), wantStatus: http.StatusOK},
		{name: "token signed with another key", token: signES256(t, other, "k1", userID), wantStatus: http.StatusUnauthorized},
		{name: "unknown kid", token: signES256(t, key, "k2", userID), wantStatus: http.StatusUnauthorized},
	}
	handler := middleware.RequireAuth(testJWTSecret, srv.URL)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id, _ := middleware.UserIDFromContext(r.Context()); id != userID {
			t.Errorf("user ID = %q", id)
		}
	}))
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", "Bearer "+tc.token)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
	// The unknown kid falls inside the one-minute refetch window.
	if n := fetches.Load(); n != 1 {
		t.Errorf("JWKS fetched %d times, want 1", n)
	}
}

func TestSupabaseJWKSURL(t *testing.T) {
	if got := middleware.SupabaseJWKSURL("https://abc.supabase.co/"); got != "https://abc.supabase.co/auth/v1/.well-known/jwks.json" {
		t.Errorf("got %q", got)
	}
	if middleware.SupabaseJWKSURL("") != "" {
		t.Error("an unset project URL should give no JWKS URL")
	}
}
