package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/natnael-alemayehu/logistics/pkg/jwt"
	"github.com/natnael-alemayehu/logistics/pkg/response"
)

type contextKey string

const (
	UserIDKey   contextKey = "userID"
	TenantIDKey contextKey = "tenantID"
	RoleKey     contextKey = "role"
	ClaimsKey   contextKey = "claims"
)

func Auth(jwtManager *jwt.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				response.ErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing authorization header")
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				response.ErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid authorization header format")
				return
			}

			claims, err := jwtManager.Validate(parts[1])
			if err != nil {
				response.ErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired token")
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, TenantIDKey, claims.TenantID)
			ctx = context.WithValue(ctx, RoleKey, claims.Role)
			ctx = context.WithValue(ctx, ClaimsKey, claims)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserID(ctx context.Context) string {
	if id, ok := ctx.Value(UserIDKey).(string); ok {
		return id
	}
	return ""
}

func GetTenantID(ctx context.Context) string {
	if id, ok := ctx.Value(TenantIDKey).(string); ok {
		return id
	}
	return ""
}

func GetRole(ctx context.Context) string {
	if role, ok := ctx.Value(RoleKey).(string); ok {
		return role
	}
	return ""
}

func GetClaims(ctx context.Context) *jwt.Claims {
	if claims, ok := ctx.Value(ClaimsKey).(*jwt.Claims); ok {
		return claims
	}
	return nil
}
