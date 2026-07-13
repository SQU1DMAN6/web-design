package apicommon

import (
	"net/http"
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
