//go:build !solution

package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type User struct {
	Name  string
	Email string
}

type contextKey struct{}

func ContextUser(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(contextKey{}).(*User)
	return u, ok
}

var ErrInvalidToken = errors.New("invalid token")

type TokenChecker interface {
	CheckToken(ctx context.Context, token string) (*User, error)
}

func CheckAuth(checker TokenChecker) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			token, _ := strings.CutPrefix(header, "Bearer ")

			user, err := checker.CheckToken(r.Context(), token)
			if errors.Is(err, ErrInvalidToken) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			ctx := context.WithValue(r.Context(), contextKey{}, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
