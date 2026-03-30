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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedUserID string
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedUserID, _ = middleware.UserIDFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware.RequireAuth(testJWTSecret)(next)
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
