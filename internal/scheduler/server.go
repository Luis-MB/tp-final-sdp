package scheduler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"sync"
	"time"

	"tp-final-sdp/internal/domain"
	"tp-final-sdp/internal/metrics"
	"tp-final-sdp/internal/repository"
	"tp-final-sdp/internal/worker"
	cryptojobsv1 "tp-final-sdp/pkg/api/cryptojobsv1"
)

const defaultChunkSize uint64 = 10000
const maxJobRanges uint64 = 1_000_000
const defaultLeaseTTL = 30 * time.Second

type Options struct {
	LeaseTTL time.Duration
	Now      func() time.Time
	Store    Store
}

type Store interface {
	LoadJobs(context.Context) ([]repository.JobSnapshot, error)
	SaveJob(context.Context, repository.JobSnapshot) error
	UpdateRange(context.Context, string, repository.RangeSnapshot) error
	UpdateJob(context.Context, string, domain.JobStatus, string) error
}

type Server struct {
	cryptojobsv1.UnimplementedSchedulerServiceServer

	mu       sync.Mutex
	jobs     map[string]*jobState
	workers  map[string]string
	leaseTTL time.Duration
	now      func() time.Time
	store    Store
}

type jobState struct {
	job             domain.Job
	plaintext       string
	totalCandidates uint64
	ranges          []rangeState
}

type rangeState struct {
	start       uint64
	end         uint64
	status      domain.RangeStatus
	workerID    string
	leasedUntil time.Time
}

func NewServer() *Server {
	return NewServerWithOptions(Options{})
}

func NewServerWithOptions(options Options) *Server {
	leaseTTL := options.LeaseTTL
	if leaseTTL <= 0 {
		leaseTTL = defaultLeaseTTL
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Server{
		jobs:     make(map[string]*jobState),
		workers:  make(map[string]string),
		leaseTTL: leaseTTL,
		now:      now,
		store:    options.Store,
	}
}

func (s *Server) LoadPersistedJobs(ctx context.Context) error {
	if s.store == nil {
		return nil
	}
	snapshots, err := s.store.LoadJobs(ctx)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, snapshot := range snapshots {
		state := &jobState{
			job:             snapshot.Job,
			plaintext:       snapshot.Plaintext,
			totalCandidates: snapshot.TotalCandidates,
			ranges:          make([]rangeState, 0, len(snapshot.Ranges)),
		}
		for _, searchRange := range snapshot.Ranges {
			state.ranges = append(state.ranges, rangeState{
				start:       searchRange.Start,
				end:         searchRange.End,
				status:      searchRange.Status,
				workerID:    searchRange.WorkerID,
				leasedUntil: searchRange.LeasedUntil,
			})
		}
		s.jobs[snapshot.Job.ID] = state
	}
	return nil
}

func (s *Server) CreateJob(ctx context.Context, req *cryptojobsv1.CreateJobRequest) (*cryptojobsv1.CreateJobResponse, error) {
	if req.GetTargetHash() == "" {
		return nil, fmt.Errorf("target_hash is required")
	}
	totalCandidates, err := worker.TotalCandidates(req.GetCharset(), req.GetMinLength(), req.GetMaxLength())
	if err != nil {
		return nil, err
	}
	if totalCandidates > math.MaxInt64 {
		return nil, fmt.Errorf("candidate space is too large: %d candidates exceeds storage limit", totalCandidates)
	}

	chunkSize := req.GetChunkSize()
	if chunkSize == 0 {
		chunkSize = defaultChunkSize
	}
	totalRanges := ((totalCandidates - 1) / chunkSize) + 1
	if totalRanges > maxJobRanges {
		return nil, fmt.Errorf("candidate space is too large: %d ranges requested, maximum is %d; increase chunk_size or reduce charset/password length", totalRanges, maxJobRanges)
	}

	ranges := make([]rangeState, 0, totalRanges)
	for start := uint64(0); start < totalCandidates; start += chunkSize {
		end := start + chunkSize
		if end > totalCandidates || end < start {
			end = totalCandidates
		}
		ranges = append(ranges, rangeState{
			start:  start,
			end:    end,
			status: domain.RangeStatusPending,
		})
	}

	jobID, err := newID()
	if err != nil {
		return nil, err
	}

	state := &jobState{
		job: domain.Job{
			ID:         jobID,
			TargetHash: req.GetTargetHash(),
			Charset:    req.GetCharset(),
			MinLength:  int(req.GetMinLength()),
			MaxLength:  int(req.GetMaxLength()),
			Status:     domain.JobStatusRunning,
		},
		totalCandidates: totalCandidates,
		ranges:          ranges,
	}
	if s.store != nil {
		if err := s.store.SaveJob(ctx, state.snapshot()); err != nil {
			return nil, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[jobID] = state

	return &cryptojobsv1.CreateJobResponse{
		JobId:           jobID,
		TotalCandidates: totalCandidates,
		TotalRanges:     uint64(len(ranges)),
	}, nil
}

func (s *Server) GetJob(_ context.Context, req *cryptojobsv1.GetJobRequest) (*cryptojobsv1.GetJobResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.jobs[req.GetJobId()]
	if !ok {
		return nil, fmt.Errorf("job not found")
	}

	return state.response(), nil
}

func (s *Server) RegisterWorker(_ context.Context, req *cryptojobsv1.RegisterWorkerRequest) (*cryptojobsv1.RegisterWorkerResponse, error) {
	if req.GetWorkerId() == "" {
		return &cryptojobsv1.RegisterWorkerResponse{Accepted: false}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.workers[req.GetWorkerId()] = req.GetHostname()
	return &cryptojobsv1.RegisterWorkerResponse{Accepted: true}, nil
}

func (s *Server) RequestRange(_ context.Context, req *cryptojobsv1.RequestRangeRequest) (*cryptojobsv1.RequestRangeResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	for _, state := range s.jobs {
		if state.job.Status != domain.JobStatusRunning {
			continue
		}
		for _, expiredRange := range state.reclaimExpiredLeases(now) {
			if err := s.persistRange(context.Background(), state.job.ID, expiredRange); err != nil {
				return nil, err
			}
		}
		for index := range state.ranges {
			if state.ranges[index].status != domain.RangeStatusPending {
				continue
			}
			state.ranges[index].status = domain.RangeStatusLeased
			state.ranges[index].workerID = req.GetWorkerId()
			state.ranges[index].leasedUntil = now.Add(s.leaseTTL)
			if err := s.persistRange(context.Background(), state.job.ID, state.ranges[index]); err != nil {
				state.ranges[index].status = domain.RangeStatusPending
				state.ranges[index].workerID = ""
				state.ranges[index].leasedUntil = time.Time{}
				return nil, err
			}
			metrics.RangesAssigned.Inc()
			return &cryptojobsv1.RequestRangeResponse{
				JobId:      state.job.ID,
				Start:      state.ranges[index].start,
				End:        state.ranges[index].end,
				HasRange:   true,
				TargetHash: state.job.TargetHash,
				Charset:    state.job.Charset,
				MinLength:  uint32(state.job.MinLength),
				MaxLength:  uint32(state.job.MaxLength),
			}, nil
		}
		state.markExhaustedIfComplete()
	}

	return &cryptojobsv1.RequestRangeResponse{}, nil
}

func (s *Server) ReportRange(_ context.Context, req *cryptojobsv1.ReportRangeRequest) (*cryptojobsv1.ReportRangeResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.jobs[req.GetJobId()]
	if !ok {
		return &cryptojobsv1.ReportRangeResponse{Accepted: false}, nil
	}

	for index := range state.ranges {
		if state.ranges[index].start == req.GetStart() && state.ranges[index].end == req.GetEnd() {
			if state.ranges[index].status == domain.RangeStatusCompleted {
				break
			}
			state.ranges[index].status = domain.RangeStatusCompleted
			state.ranges[index].workerID = req.GetWorkerId()
			state.ranges[index].leasedUntil = time.Time{}
			if err := s.persistRange(context.Background(), state.job.ID, state.ranges[index]); err != nil {
				return nil, err
			}
			metrics.RangesCompleted.Inc()
			break
		}
	}

	if req.GetFound() && state.job.Status == domain.JobStatusRunning {
		state.job.Status = domain.JobStatusFound
		state.plaintext = req.GetPlaintext()
		if err := s.persistJob(context.Background(), state); err != nil {
			return nil, err
		}
		metrics.JobsFound.Inc()
		return &cryptojobsv1.ReportRangeResponse{Accepted: true, ShouldStop: true}, nil
	}

	previousStatus := state.job.Status
	state.markExhaustedIfComplete()
	if state.job.Status != previousStatus {
		if err := s.persistJob(context.Background(), state); err != nil {
			return nil, err
		}
	}
	return &cryptojobsv1.ReportRangeResponse{
		Accepted:   true,
		ShouldStop: state.job.Status == domain.JobStatusFound,
	}, nil
}

func (s *jobState) snapshot() repository.JobSnapshot {
	snapshot := repository.JobSnapshot{
		Job:             s.job,
		Plaintext:       s.plaintext,
		TotalCandidates: s.totalCandidates,
		Ranges:          make([]repository.RangeSnapshot, 0, len(s.ranges)),
	}
	for _, searchRange := range s.ranges {
		snapshot.Ranges = append(snapshot.Ranges, repository.RangeSnapshot{
			Start:       searchRange.start,
			End:         searchRange.end,
			Status:      searchRange.status,
			WorkerID:    searchRange.workerID,
			LeasedUntil: searchRange.leasedUntil,
		})
	}
	return snapshot
}

func (s *Server) persistRange(ctx context.Context, jobID string, searchRange rangeState) error {
	if s.store == nil {
		return nil
	}
	return s.store.UpdateRange(ctx, jobID, repository.RangeSnapshot{
		Start:       searchRange.start,
		End:         searchRange.end,
		Status:      searchRange.status,
		WorkerID:    searchRange.workerID,
		LeasedUntil: searchRange.leasedUntil,
	})
}

func (s *Server) persistJob(ctx context.Context, state *jobState) error {
	if s.store == nil {
		return nil
	}
	return s.store.UpdateJob(ctx, state.job.ID, state.job.Status, state.plaintext)
}

func (s *jobState) response() *cryptojobsv1.GetJobResponse {
	return &cryptojobsv1.GetJobResponse{
		JobId:           s.job.ID,
		TargetHash:      s.job.TargetHash,
		Charset:         s.job.Charset,
		MinLength:       uint32(s.job.MinLength),
		MaxLength:       uint32(s.job.MaxLength),
		Status:          string(s.job.Status),
		Plaintext:       s.plaintext,
		CompletedRanges: s.completedRanges(),
		TotalRanges:     uint64(len(s.ranges)),
		TotalCandidates: s.totalCandidates,
	}
}

func (s *jobState) completedRanges() uint64 {
	var completed uint64
	for _, searchRange := range s.ranges {
		if searchRange.status == domain.RangeStatusCompleted {
			completed++
		}
	}
	return completed
}

func (s *jobState) reclaimExpiredLeases(now time.Time) []rangeState {
	var expired []rangeState
	for index := range s.ranges {
		searchRange := &s.ranges[index]
		if searchRange.status != domain.RangeStatusLeased {
			continue
		}
		if searchRange.leasedUntil.IsZero() || now.Before(searchRange.leasedUntil) {
			continue
		}
		searchRange.status = domain.RangeStatusPending
		searchRange.workerID = ""
		searchRange.leasedUntil = time.Time{}
		expired = append(expired, *searchRange)
		metrics.RangesExpired.Inc()
	}
	return expired
}

func (s *jobState) markExhaustedIfComplete() {
	if s.job.Status != domain.JobStatusRunning {
		return
	}
	for _, searchRange := range s.ranges {
		if searchRange.status != domain.RangeStatusCompleted {
			return
		}
	}
	s.job.Status = domain.JobStatusExhausted
}

func newID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}
