package user

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/SAYAN02-DEV/iron-bench/internal/db"
	"github.com/SAYAN02-DEV/iron-bench/internal/db/sqlc"
	"github.com/SAYAN02-DEV/iron-bench/internal/response"
	"github.com/SAYAN02-DEV/iron-bench/internal/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

func Signup() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req types.User
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if err := response.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"}); err != nil {
				log.Println("write response error:", err)
			}
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			log.Println("hash password error:", err)
			if err := response.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create user"}); err != nil {
				log.Println("write response error:", err)
			}
			return
		}

		q := sqlc.New(db.Pool)
		created, err := q.CreateUser(r.Context(), sqlc.CreateUserParams{
			Username: req.Username,
			Email:    req.Email,
			Password: string(hashedPassword),
		})
		if err != nil {
			log.Println("create user error:", err)
			if err := response.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create user"}); err != nil {
				log.Println("write response error:", err)
			}
			return
		}

		log.Printf("user created: %s (%s)", created.Username, created.Email)

		if err := response.WriteJSON(w, http.StatusCreated, created); err != nil {
			log.Println("write response error:", err)
		}
	}
}

func Signin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req types.User
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if err := response.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"}); err != nil {
				log.Println("write response error:", err)
			}
			return
		}

		q := sqlc.New(db.Pool)
		user, err := q.GetUserByEmail(r.Context(), req.Email)
		if err != nil {
			if err == pgx.ErrNoRows {
				if err := response.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"}); err != nil {
					log.Println("write response error:", err)
				}
				return
			}
			log.Println("get user error:", err)
			if err := response.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to signin"}); err != nil {
				log.Println("write response error:", err)
			}
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
			if err := response.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"}); err != nil {
				log.Println("write response error:", err)
			}
			return
		}

		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			log.Println("JWT_SECRET is not set")
			if err := response.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to signin"}); err != nil {
				log.Println("write response error:", err)
			}
			return
		}

		claims := jwt.MapClaims{
			"sub":   user.ID,
			"email": user.Email,
			"exp":   time.Now().Add(24 * time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, err := token.SignedString([]byte(secret))
		if err != nil {
			log.Println("sign token error:", err)
			if err := response.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to signin"}); err != nil {
				log.Println("write response error:", err)
			}
			return
		}

		if err := response.WriteJSON(w, http.StatusOK, map[string]string{"token": signed}); err != nil {
			log.Println("write response error:", err)
		}
	}
}
