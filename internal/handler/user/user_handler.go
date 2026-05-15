package user

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/SAYAN02-DEV/iron-bench/internal/response"
	"github.com/SAYAN02-DEV/iron-bench/internal/types"
)

func New() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var user types.User
		err := json.NewDecoder(r.Body).Decode(&user)
		if err != nil {
			log.Println(err)
		}
		if errors.Is(err, io.EOF) {
			response.WriteJson(w, http.StatusBadRequest, err.Error())
			return
		}

		log.Printf("user signup: %s", user.Username)

		response.WriteJson(w, http.StatusCreated, map[string]string{"success": "user created"})
	}
}
