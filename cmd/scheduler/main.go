package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"

	"tp-final-sdp/internal/config"
	"tp-final-sdp/internal/metrics"
	"tp-final-sdp/internal/repository"
	"tp-final-sdp/internal/scheduler"
	cryptojobsv1 "tp-final-sdp/pkg/api/cryptojobsv1"

	"google.golang.org/grpc"
)

func main() {
	cfg := config.Load()
	metrics.Register()

	var store scheduler.Store
	if cfg.DatabaseURL != "" {
		postgresStore, err := openPostgresWithRetry(cfg.DatabaseURL, 30*time.Second)
		if err != nil {
			log.Fatalf("failed to connect PostgreSQL: %v", err)
		}
		defer postgresStore.Close()
		store = postgresStore
		log.Print("scheduler persistence enabled with PostgreSQL")
	}

	listener, err := net.Listen("tcp", cfg.SchedulerGRPCAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	schedulerServer := scheduler.NewServerWithOptions(scheduler.Options{
		LeaseTTL: cfg.RangeLeaseTTL,
		Store:    store,
	})
	if err := schedulerServer.LoadPersistedJobs(context.Background()); err != nil {
		log.Fatalf("failed to load persisted jobs: %v", err)
	}

	server := grpc.NewServer()
	cryptojobsv1.RegisterSchedulerServiceServer(server, schedulerServer)

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", metrics.Handler())
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
		log.Printf("scheduler metrics listening on %s", cfg.SchedulerMetricsAddr)
		if err := http.ListenAndServe(cfg.SchedulerMetricsAddr, mux); err != nil {
			log.Fatal(err)
		}
	}()

	log.Printf("scheduler grpc listening on %s", cfg.SchedulerGRPCAddr)
	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}

func openPostgresWithRetry(databaseURL string, timeout time.Duration) (*repository.PostgresStore, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		store, err := repository.OpenPostgres(ctx, databaseURL)
		cancel()
		if err == nil {
			return store, nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return nil, lastErr
		}
		log.Printf("waiting for PostgreSQL: %v", err)
		time.Sleep(time.Second)
	}
}
