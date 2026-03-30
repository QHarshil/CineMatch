package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const contextKeyUserID contextKey = "userID"

// RequireAuth returns a middleware that verifies the Supabase JWT and injects the user UUID
// into the request context. jwtSecret is captured once at server startup so we avoid
// an os.Getenv call on every authenticated request.
func RequireAuth(jwtSecret string) func(http.Handler) http.Handler {
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

// extractBearerToken parses "Authorization: Bearer <token>" from the request.
func extractBearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return "", false
	}
	token := strings.TrimPrefix(header, "Bearer ")
	return token, token != ""
}

// verifySupabaseJWT validates a Supabase-issued HS256 JWT and returns the subject (user UUID).
// Supabase signs JWTs with the project's JWT secret, which must match JWT_SECRET in env.
func verifySupabaseJWT(tokenString, secret string) (string, error) {
	parsed, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return "", err
	}

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
