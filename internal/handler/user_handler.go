package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/SAYAN02-DEV/iron-bench/internal/db"
	"github.com/SAYAN02-DEV/iron-bench/internal/db/sqlc"
	"github.com/SAYAN02-DEV/iron-bench/internal/response"
	"github.com/SAYAN02-DEV/iron-bench/internal/types"
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
