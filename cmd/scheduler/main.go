package main

import (
	"log"
	"net"
	"net/http"

	"tp-final-sdp/internal/config"
	"tp-final-sdp/internal/metrics"
	"tp-final-sdp/internal/scheduler"
	cryptojobsv1 "tp-final-sdp/pkg/api/cryptojobsv1"

	"google.golang.org/grpc"
)

func main() {
	cfg := config.Load()
	metrics.Register()

	listener, err := net.Listen("tcp", cfg.SchedulerGRPCAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	server := grpc.NewServer()
	cryptojobsv1.RegisterSchedulerServiceServer(server, scheduler.NewServerWithOptions(scheduler.Options{
		LeaseTTL: cfg.RangeLeaseTTL,
	}))

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
