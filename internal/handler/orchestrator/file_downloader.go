package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	aws_s3 "github.com/SAYAN02-DEV/iron-bench/libs/aws-s3"
)

type FileRequest struct {
	FileName string `json:"fileName"`
	UserID   string `json:"userId"`
}

type Orchestrator struct {
	Presigner aws_s3.Presigner
	Bucket    string
}

func (o *Orchestrator) downloadFilesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		presigner := o.Presigner
		bucket := o.Bucket
		// Parse request body
		var files []FileRequest
		if err := json.NewDecoder(r.Body).Decode(&files); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		// Download each file
		downloadDir := "./downloads" // where files will be saved
		os.MkdirAll(downloadDir, os.ModePerm)

		results := []map[string]string{}

		for _, f := range files {
			// Generate presigned URL
			getReq, err := presigner.GetObject(context.TODO(), bucket, f.FileName, 3600)
			if err != nil {
				log.Println("Error generating URL for:", f.FileName, err)
				results = append(results, map[string]string{
					"fileName": f.FileName,
					"userId":   f.UserID,
					"status":   "failed",
					"error":    err.Error(),
				})
				continue
			}

			// Download the file
			resp, err := http.Get(getReq.URL)
			if err != nil {
				log.Println("Error downloading:", f.FileName, err)
				results = append(results, map[string]string{
					"fileName": f.FileName,
					"userId":   f.UserID,
					"status":   "failed",
					"error":    err.Error(),
				})
				continue
			}
			defer resp.Body.Close()

			// Save to local folder as userId_fileName
			savePath := filepath.Join(downloadDir, fmt.Sprintf("%s_%s", f.UserID, f.FileName))
			outFile, err := os.Create(savePath)
			if err != nil {
				log.Println("Error creating file:", savePath, err)
				continue
			}
			defer outFile.Close()

			io.Copy(outFile, resp.Body)

			log.Printf("Saved: %s\n", savePath)
			results = append(results, map[string]string{
				"fileName": f.FileName,
				"userId":   f.UserID,
				"status":   "success",
				"path":     savePath,
			})
		}

		// Respond with results
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

func DownloadFilesHandler() http.HandlerFunc {
	presigner, cfg, err := aws_s3.NewPresignerFromEnv()
	if err != nil {
		log.Printf("failed to init presigner: %v", err)
		return func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "s3 presigner not configured", http.StatusInternalServerError)
		}
	}

	o := &Orchestrator{
		Presigner: presigner,
		Bucket:    cfg.Bucket,
	}
	return o.downloadFilesHandler()
}
