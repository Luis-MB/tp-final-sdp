package config

import (
	"os"
	"runtime"
	"strconv"
	"time"
)

type Config struct {
	APIHTTPAddr          string
	SchedulerGRPCAddr    string
	SchedulerMetricsAddr string
	DatabaseURL          string
	APIToken             string
	WorkerID             string
	WorkerConcurrency    int
	RangeLeaseTTL        time.Duration
}

func Load() Config {
	return Config{
		APIHTTPAddr:          env("API_HTTP_ADDR", ":8080"),
		SchedulerGRPCAddr:    env("SCHEDULER_GRPC_ADDR", ":9090"),
		SchedulerMetricsAddr: env("SCHEDULER_METRICS_ADDR", ":9100"),
		DatabaseURL:          env("DATABASE_URL", ""),
		APIToken:             env("API_TOKEN", ""),
		WorkerID:             env("WORKER_ID", ""),
		WorkerConcurrency:    envInt("WORKER_CONCURRENCY", runtime.NumCPU()),
		RangeLeaseTTL:        envDuration("SCHEDULER_RANGE_LEASE_TTL", 30*time.Second),
	}
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return duration
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}
