package scheduler

import (
	"context"
	"testing"
	"time"

	"tp-final-sdp/internal/crypto"
	cryptojobsv1 "tp-final-sdp/pkg/api/cryptojobsv1"
)

func TestSchedulerCreateAssignReportFound(t *testing.T) {
	server := NewServer()
	ctx := context.Background()

	createResp, err := server.CreateJob(ctx, &cryptojobsv1.CreateJobRequest{
		TargetHash: crypto.SHA256Hex("ab"),
		Charset:    "ab",
		MinLength:  1,
		MaxLength:  2,
		ChunkSize:  2,
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	if createResp.GetTotalCandidates() != 6 {
		t.Fatalf("TotalCandidates = %d, want 6", createResp.GetTotalCandidates())
	}
	if createResp.GetTotalRanges() != 3 {
		t.Fatalf("TotalRanges = %d, want 3", createResp.GetTotalRanges())
	}

	rangeResp, err := server.RequestRange(ctx, &cryptojobsv1.RequestRangeRequest{WorkerId: "worker-1"})
	if err != nil {
		t.Fatalf("RequestRange() error = %v", err)
	}
	if !rangeResp.GetHasRange() {
		t.Fatal("expected a range")
	}

	_, err = server.ReportRange(ctx, &cryptojobsv1.ReportRangeRequest{
		WorkerId:  "worker-1",
		JobId:     createResp.GetJobId(),
		Start:     rangeResp.GetStart(),
		End:       rangeResp.GetEnd(),
		Found:     true,
		Plaintext: "ab",
	})
	if err != nil {
		t.Fatalf("ReportRange() error = %v", err)
	}

	jobResp, err := server.GetJob(ctx, &cryptojobsv1.GetJobRequest{JobId: createResp.GetJobId()})
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if jobResp.GetStatus() != "found" {
		t.Fatalf("Status = %q, want found", jobResp.GetStatus())
	}
	if jobResp.GetPlaintext() != "ab" {
		t.Fatalf("Plaintext = %q, want ab", jobResp.GetPlaintext())
	}
}

func TestSchedulerReassignsExpiredLease(t *testing.T) {
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	server := NewServerWithOptions(Options{
		LeaseTTL: time.Second,
		Now: func() time.Time {
			return now
		},
	})
	ctx := context.Background()

	createResp, err := server.CreateJob(ctx, &cryptojobsv1.CreateJobRequest{
		TargetHash: crypto.SHA256Hex("ab"),
		Charset:    "ab",
		MinLength:  1,
		MaxLength:  2,
		ChunkSize:  2,
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	firstLease, err := server.RequestRange(ctx, &cryptojobsv1.RequestRangeRequest{WorkerId: "worker-1"})
	if err != nil {
		t.Fatalf("RequestRange() error = %v", err)
	}
	if !firstLease.GetHasRange() {
		t.Fatal("expected first worker to receive a range")
	}

	now = now.Add(2 * time.Second)
	secondLease, err := server.RequestRange(ctx, &cryptojobsv1.RequestRangeRequest{WorkerId: "worker-2"})
	if err != nil {
		t.Fatalf("RequestRange() error = %v", err)
	}
	if !secondLease.GetHasRange() {
		t.Fatal("expected second worker to receive an expired range")
	}
	if secondLease.GetJobId() != createResp.GetJobId() {
		t.Fatalf("JobId = %q, want %q", secondLease.GetJobId(), createResp.GetJobId())
	}
	if secondLease.GetStart() != firstLease.GetStart() || secondLease.GetEnd() != firstLease.GetEnd() {
		t.Fatalf("range = [%d,%d), want [%d,%d)", secondLease.GetStart(), secondLease.GetEnd(), firstLease.GetStart(), firstLease.GetEnd())
	}
}

func TestSchedulerRejectsTooManyRanges(t *testing.T) {
	server := NewServer()
	ctx := context.Background()

	_, err := server.CreateJob(ctx, &cryptojobsv1.CreateJobRequest{
		TargetHash: crypto.SHA256Hex("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		Charset:    "ab",
		MinLength:  40,
		MaxLength:  40,
		ChunkSize:  1,
	})
	if err == nil {
		t.Fatal("expected CreateJob() to reject too many ranges")
	}
}
