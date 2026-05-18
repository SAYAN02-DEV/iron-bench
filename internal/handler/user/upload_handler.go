package user

import (
	"net/http"
	"time"

	"github.com/SAYAN02-DEV/iron-bench/internal/response"
	aws_s3 "github.com/SAYAN02-DEV/iron-bench/libs/aws-s3"
)

type UploadResponse struct {
	URL string `json:"url"`
}

func GetUploadURL() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		presigner, cfg, err := aws_s3.NewPresignerFromEnv()
		if err != nil {
			_ = response.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		req, err := presigner.PutObject(r.Context(), cfg.Bucket, cfg.ObjectKey, int64(time.Duration(cfg.ExpiresSeconds).Seconds()))
		if err != nil {
			_ = response.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		_ = response.WriteJSON(w, http.StatusOK, UploadResponse{URL: req.URL})
	}
}
