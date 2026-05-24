package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"inkdrop/config"
)

func keyNames(kind string) (attemptKey, lastKey, lockKey string) {
	attemptKey = fmt.Sprintf("%sAttempts", kind)
	lastKey = fmt.Sprintf("%sLastTry", kind)
	lockKey = fmt.Sprintf("%sLockUntil", kind)
	return
}

func AllowAttempt(r *http.Request, kind string, maxAttempts int, window, lockDuration time.Duration) (bool, string) {
	SS := config.GetSessionManager()
	attemptKey, lastKey, lockKey := keyNames(kind)
	now := time.Now()

	if lockValue := SS.GetInt64(r.Context(), lockKey); lockValue > 0 {
		lockUntil := time.Unix(lockValue, 0)
		if now.Before(lockUntil) {
			wait := lockUntil.Sub(now).Round(time.Second)
			if wait < time.Second {
				wait = time.Second
			}
			return false, fmt.Sprintf("Too many %s attempts. Please wait %s and try again.", kind, wait)
		}
	}

	lastTry := time.Unix(SS.GetInt64(r.Context(), lastKey), 0)
	attempts := SS.GetInt(r.Context(), attemptKey)
	if now.Sub(lastTry) > window {
		attempts = 0
	}

	attempts++
	SS.Put(r.Context(), attemptKey, attempts)
	SS.Put(r.Context(), lastKey, now.Unix())

	if attempts > maxAttempts {
		SS.Put(r.Context(), lockKey, now.Add(lockDuration).Unix())
		return false, fmt.Sprintf("Too many %s attempts. Please wait %d seconds before trying again.", kind, int(lockDuration.Seconds()))
	}

	return true, ""
}

func ClearAttempts(r *http.Request, kind string) {
	SS := config.GetSessionManager()
	attemptKey, lastKey, lockKey := keyNames(kind)
	SS.Remove(r.Context(), attemptKey)
	SS.Remove(r.Context(), lastKey)
	SS.Remove(r.Context(), lockKey)
}

func FormatKind(kind string) string {
	kind = strings.TrimSpace(strings.ToLower(kind))
	switch kind {
	case "login":
		return "login"
	case "register", "registration":
		return "registration"
	default:
		return kind
	}
}
