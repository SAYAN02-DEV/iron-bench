package middleware

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/SAYAN02-DEV/iron-bench/internal/response"
	"github.com/golang-jwt/jwt/v5"
)

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			_ = response.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing authorization"})
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			_ = response.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid authorization"})
			return
		}

		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			log.Println("JWT_SECRET is not set")
			_ = response.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "server misconfigured"})
			return
		}

		tokenStr := parts[1]
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrTokenSignatureInvalid
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			_ = response.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			return
		}

		next.ServeHTTP(w, r)
	})
}
