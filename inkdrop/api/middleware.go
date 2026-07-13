package api

import (
	"context"
	"net/http"

	"inkdrop/apicommon"
	"inkdrop/config"
	"inkdrop/model"
	"inkdrop/service"
)

// contextKey type for request context values.
type contextKey string

const (
	// UserIDKey is the context key for the authenticated user ID.
	UserIDKey contextKey = "user_id"
	// UserNameKey is the context key for the authenticated user name.
	UserNameKey contextKey = "user_name"
	// DropIDKey is the context key for the current drop ID.
	DropIDKey contextKey = "drop_id"
)

// AuthMiddleware verifies the user is authenticated and adds user info to context.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ss := config.GetSessionManager()
		isLoggedIn := ss.GetBool(r.Context(), "isLoggedIn")
		userName := ss.GetString(r.Context(), "name")

		if !isLoggedIn || userName == "" {
			apicommon.WriteError(w, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required")
			return
		}

		// Get user ID
		db := config.GetDB()
		user, err := model.GetUserByName(userName, db)
		if err != nil {
			apicommon.WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found")
			return
		}

		// Store user info in context
		ctx := r.Context()
		ctx = context.WithValue(ctx, UserIDKey, user.ID)
		ctx = context.WithValue(ctx, UserNameKey, user.Name)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequirePermission returns middleware that checks the user has at least the required role.
func RequirePermission(permService *service.PermissionService, requiredRole model.DropRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			dropID := r.Context().Value(DropIDKey)
			if dropID == nil {
				// Try to get from URL param
				dropID = r.PathValue("id")
				if dropID == "" {
					apicommon.WriteError(w, http.StatusBadRequest, "MISSING_DROP", "Drop ID required")
					return
				}
			}

			userID, ok := r.Context().Value(UserIDKey).(int64)
			if !ok {
				apicommon.WriteError(w, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required")
				return
			}

			dropIDStr, ok := dropID.(string)
			if !ok {
				apicommon.WriteError(w, http.StatusBadRequest, "INVALID_DROP", "Invalid drop ID")
				return
			}

			if !permService.CheckPermission(userID, dropIDStr, requiredRole) {
				apicommon.WriteError(w, http.StatusForbidden, "PERMISSION_DENIED", "Insufficient permissions")
				return
			}

			// Store drop ID in context
			ctx := context.WithValue(r.Context(), DropIDKey, dropIDStr)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID extracts the user ID from the request context.
func GetUserID(r *http.Request) int64 {
	if id, ok := r.Context().Value(UserIDKey).(int64); ok {
		return id
	}
	return 0
}

// GetUserName extracts the user name from the request context.
func GetUserName(r *http.Request) string {
	if name, ok := r.Context().Value(UserNameKey).(string); ok {
		return name
	}
	return ""
}

// GetDropID extracts the drop ID from the request context.
func GetDropID(r *http.Request) string {
	if id, ok := r.Context().Value(DropIDKey).(string); ok {
		return id
	}
	return ""
}
