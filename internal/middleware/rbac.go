package middleware

import (
	"net/http"

	"github.com/natnael-alemayehu/logistics/pkg/response"
)

func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := GetRole(r.Context())
			if role == "" {
				response.ErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "No role found in context")
				return
			}

			for _, allowed := range allowedRoles {
				if role == allowed {
					next.ServeHTTP(w, r)
					return
				}
			}

			response.ErrorJSON(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		})
	}
}

func RequireDriver() func(http.Handler) http.Handler {
	return RequireRole("driver")
}

func RequireDispatcher() func(http.Handler) http.Handler {
	return RequireRole("dispatcher", "fleet_manager", "admin")
}

func RequireAdmin() func(http.Handler) http.Handler {
	return RequireRole("admin", "platform_admin")
}
