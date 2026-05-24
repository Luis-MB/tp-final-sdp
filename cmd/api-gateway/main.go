package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"tp-final-sdp/internal/config"
	internalcrypto "tp-final-sdp/internal/crypto"
	"tp-final-sdp/internal/metrics"
	cryptojobsv1 "tp-final-sdp/pkg/api/cryptojobsv1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	defaultMinLength = 1
	defaultMaxLength = 6
)

type createJobRequest struct {
	TargetHash string `json:"target_hash"`
	Password   string `json:"password"`
	Charset    string `json:"charset"`
	MinLength  uint32 `json:"min_length"`
	MaxLength  uint32 `json:"max_length"`
	ChunkSize  uint64 `json:"chunk_size"`
}

func main() {
	cfg := config.Load()
	metrics.Register()

	conn, err := grpc.NewClient(cfg.SchedulerGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	schedulerClient := cryptojobsv1.NewSchedulerServiceClient(conn)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/jobs", requireToken(cfg.APIToken, handleJobs(schedulerClient)))
	mux.HandleFunc("/jobs/", requireToken(cfg.APIToken, handleJob(schedulerClient)))
	mux.Handle("/metrics", metrics.Handler())

	log.Printf("api-gateway listening on %s", cfg.APIHTTPAddr)
	if err := http.ListenAndServe(cfg.APIHTTPAddr, mux); err != nil {
		log.Fatal(err)
	}
}

func requireToken(token string, next http.HandlerFunc) http.HandlerFunc {
	if token == "" {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func handleJobs(client cryptojobsv1.SchedulerServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req createJobRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		normalizeCreateJobRequest(&req)

		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		resp, err := client.CreateJob(ctx, &cryptojobsv1.CreateJobRequest{
			TargetHash: req.TargetHash,
			Charset:    req.Charset,
			MinLength:  req.MinLength,
			MaxLength:  req.MaxLength,
			ChunkSize:  req.ChunkSize,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		metrics.JobsCreated.Inc()
		writeJSON(w, http.StatusCreated, resp)
	}
}

func normalizeCreateJobRequest(req *createJobRequest) {
	if req.Password != "" {
		req.TargetHash = internalcrypto.SHA256Hex(req.Password)
		passwordLength := uint32(len(req.Password))
		if req.MinLength == 0 {
			req.MinLength = passwordLength
		}
		if req.MaxLength == 0 {
			req.MaxLength = passwordLength
		}
	}
	if req.MinLength == 0 {
		req.MinLength = defaultMinLength
	}
	if req.MaxLength == 0 {
		req.MaxLength = defaultMaxLength
	}
}

func handleJob(client cryptojobsv1.SchedulerServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		jobID := strings.TrimPrefix(r.URL.Path, "/jobs/")
		if jobID == "" {
			http.Error(w, "job id is required", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		resp, err := client.GetJob(ctx, &cryptojobsv1.GetJobRequest{JobId: jobID})
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
