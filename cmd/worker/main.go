package main

import (
	"context"
	"log"
	"os"
	"time"

	"tp-final-sdp/internal/config"
	searchworker "tp-final-sdp/internal/worker"
	cryptojobsv1 "tp-final-sdp/pkg/api/cryptojobsv1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	cfg := config.Load()
	hostname, _ := os.Hostname()
	if cfg.WorkerID == "" {
		cfg.WorkerID = hostname
	}
	if cfg.WorkerID == "" {
		cfg.WorkerID = "worker-local"
	}
	log.Printf("worker %s starting; scheduler=%s redis=%s concurrency=%d", cfg.WorkerID, cfg.SchedulerGRPCAddr, cfg.RedisAddr, cfg.WorkerConcurrency)

	conn, err := grpc.NewClient(cfg.SchedulerGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := cryptojobsv1.NewSchedulerServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	registerResp, err := client.RegisterWorker(ctx, &cryptojobsv1.RegisterWorkerRequest{
		WorkerId: cfg.WorkerID,
		Hostname: hostname,
	})
	cancel()
	if err != nil {
		log.Fatal(err)
	}
	if !registerResp.GetAccepted() {
		log.Fatalf("worker %s was rejected", cfg.WorkerID)
	}

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		rangeResp, err := client.RequestRange(ctx, &cryptojobsv1.RequestRangeRequest{WorkerId: cfg.WorkerID})
		cancel()
		if err != nil {
			log.Printf("worker %s failed to request range: %v", cfg.WorkerID, err)
			time.Sleep(time.Second)
			continue
		}
		if rangeResp.GetShouldStop() || !rangeResp.GetHasRange() {
			time.Sleep(time.Second)
			continue
		}

		startedAt := time.Now()
		log.Printf(
			"worker %s processing job %s range [%d,%d)",
			cfg.WorkerID,
			rangeResp.GetJobId(),
			rangeResp.GetStart(),
			rangeResp.GetEnd(),
		)

		result := searchworker.SearchSHA256RangeParallel(
			rangeResp.GetTargetHash(),
			rangeResp.GetCharset(),
			rangeResp.GetMinLength(),
			rangeResp.GetMaxLength(),
			searchworker.Range{Start: rangeResp.GetStart(), End: rangeResp.GetEnd()},
			cfg.WorkerConcurrency,
		)

		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		reportResp, err := client.ReportRange(ctx, &cryptojobsv1.ReportRangeRequest{
			WorkerId:  cfg.WorkerID,
			JobId:     rangeResp.GetJobId(),
			Start:     rangeResp.GetStart(),
			End:       rangeResp.GetEnd(),
			Found:     result.Found,
			Plaintext: result.Plaintext,
		})
		cancel()
		if err != nil {
			log.Printf("worker %s failed to report range: %v", cfg.WorkerID, err)
			time.Sleep(time.Second)
			continue
		}
		log.Printf(
			"worker %s completed job %s range [%d,%d) found=%t duration=%s",
			cfg.WorkerID,
			rangeResp.GetJobId(),
			rangeResp.GetStart(),
			rangeResp.GetEnd(),
			result.Found,
			time.Since(startedAt).Round(time.Millisecond),
		)
		if result.Found {
			log.Printf("worker %s found plaintext for job %s: %s", cfg.WorkerID, rangeResp.GetJobId(), result.Plaintext)
		}
		if reportResp.GetShouldStop() {
			time.Sleep(time.Second)
		}
	}
}
