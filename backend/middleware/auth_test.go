package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/harshilc/cinematch-backend/middleware"
)

const testJWTSecret = "test-secret-32-chars-long-padding"

func makeSupabaseJWT(t *testing.T, userID string, secret string, expiry time.Duration) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":  userID,
		"role": "authenticated",
		"aud":  "authenticated",
		"exp":  time.Now().Add(expiry).Unix(),
		"iat":  time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("signing test JWT: %v", err)
	}
	return signed
}

func signClaims(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func TestRequireAuth(t *testing.T) {
	validUserID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
		wantUserID string
	}{
		{
			name:       "valid token passes and injects user ID",
			authHeader: "Bearer " + makeSupabaseJWT(t, validUserID, testJWTSecret, time.Hour),
			wantStatus: http.StatusOK,
			wantUserID: validUserID,
		},
		{
			name:       "missing header returns 401",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "malformed bearer value returns 401",
			authHeader: "Bearer not.a.jwt",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "expired token returns 401",
			authHeader: "Bearer " + makeSupabaseJWT(t, validUserID, testJWTSecret, -time.Hour),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "wrong secret returns 401",
			authHeader: "Bearer " + makeSupabaseJWT(t, validUserID, "wrong-secret-padding-padding", time.Hour),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "token for another audience returns 401",
			authHeader: "Bearer " + signClaims(t, jwt.MapClaims{"sub": validUserID, "aud": "anon", "exp": time.Now().Add(time.Hour).Unix()}),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "token without an expiry returns 401",
			authHeader: "Bearer " + signClaims(t, jwt.MapClaims{"sub": validUserID, "aud": "authenticated"}),
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		// Also test X-Authorization fallback for each case with a header
		if tc.authHeader != "" {
			t.Run(tc.name+" via X-Authorization", func(t *testing.T) {
				var capturedUserID string
				next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					capturedUserID, _ = middleware.UserIDFromContext(r.Context())
					w.WriteHeader(http.StatusOK)
				})

				handler := middleware.RequireAuth(testJWTSecret, "")(next)
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("X-Authorization", tc.authHeader)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != tc.wantStatus {
					t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
				}
				if tc.wantUserID != "" && capturedUserID != tc.wantUserID {
					t.Errorf("userID in context = %q, want %q", capturedUserID, tc.wantUserID)
				}
			})
		}

		t.Run(tc.name, func(t *testing.T) {
			var capturedUserID string
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedUserID, _ = middleware.UserIDFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware.RequireAuth(testJWTSecret, "")(next)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantUserID != "" && capturedUserID != tc.wantUserID {
				t.Errorf("userID in context = %q, want %q", capturedUserID, tc.wantUserID)
			}
		})
	}
}

func TestRequireAuthMarksGuestSessions(t *testing.T) {
	const secret = "test-secret"
	for _, tc := range []struct {
		name      string
		anonymous any
		wantGuest bool
	}{
		{name: "anonymous sign-in", anonymous: true, wantGuest: true},
		{name: "email sign-in", anonymous: false, wantGuest: false},
		{name: "claim absent", anonymous: nil, wantGuest: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := jwt.MapClaims{"sub": "11111111-1111-1111-1111-111111111111", "aud": "authenticated", "exp": time.Now().Add(time.Hour).Unix()}
			if tc.anonymous != nil {
				claims["is_anonymous"] = tc.anonymous
			}
			signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
			if err != nil {
				t.Fatal(err)
			}

			var gotGuest bool
			handler := middleware.RequireAuth(secret, "")(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				gotGuest = middleware.IsGuestFromContext(r.Context())
			}))
			req := httptest.NewRequest(http.MethodGet, "/assistant/usage", nil)
			req.Header.Set("Authorization", "Bearer "+signed)
			handler.ServeHTTP(httptest.NewRecorder(), req)

			if gotGuest != tc.wantGuest {
				t.Errorf("guest = %v, want %v", gotGuest, tc.wantGuest)
			}
		})
	}
}
