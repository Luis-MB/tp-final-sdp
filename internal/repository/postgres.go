package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"tp-final-sdp/internal/domain"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type JobSnapshot struct {
	Job             domain.Job
	Plaintext       string
	TotalCandidates uint64
	Ranges          []RangeSnapshot
}

type RangeSnapshot struct {
	Start       uint64
	End         uint64
	Status      domain.RangeStatus
	WorkerID    string
	LeasedUntil time.Time
}

type PostgresStore struct {
	db *sql.DB
}

func OpenPostgres(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	store := &PostgresStore{db: db}
	if err := store.ensureSchema(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}

func (s *PostgresStore) ensureSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			target_hash TEXT NOT NULL,
			charset TEXT NOT NULL,
			min_length INTEGER NOT NULL,
			max_length INTEGER NOT NULL,
			status TEXT NOT NULL,
			plaintext TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS search_ranges (
			id BIGSERIAL PRIMARY KEY,
			job_id TEXT NOT NULL REFERENCES jobs(id),
			range_start BIGINT NOT NULL,
			range_end BIGINT NOT NULL,
			status TEXT NOT NULL,
			worker_id TEXT,
			leased_until TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_search_ranges_job_status
			ON search_ranges(job_id, status)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_search_ranges_job_bounds
			ON search_ranges(job_id, range_start, range_end)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) LoadJobs(ctx context.Context) ([]JobSnapshot, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, target_hash, charset, min_length, max_length, status, COALESCE(plaintext, '')
		FROM jobs
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []JobSnapshot
	for rows.Next() {
		var snapshot JobSnapshot
		var status string
		if err := rows.Scan(
			&snapshot.Job.ID,
			&snapshot.Job.TargetHash,
			&snapshot.Job.Charset,
			&snapshot.Job.MinLength,
			&snapshot.Job.MaxLength,
			&status,
			&snapshot.Plaintext,
		); err != nil {
			return nil, err
		}
		snapshot.Job.Status = domain.JobStatus(status)
		jobs = append(jobs, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for index := range jobs {
		ranges, totalCandidates, err := s.loadRanges(ctx, jobs[index].Job.ID)
		if err != nil {
			return nil, err
		}
		jobs[index].Ranges = ranges
		jobs[index].TotalCandidates = totalCandidates
	}
	return jobs, nil
}

func (s *PostgresStore) loadRanges(ctx context.Context, jobID string) ([]RangeSnapshot, uint64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT range_start, range_end, status, COALESCE(worker_id, ''), leased_until
		FROM search_ranges
		WHERE job_id = $1
		ORDER BY range_start ASC
	`, jobID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var ranges []RangeSnapshot
	var totalCandidates uint64
	for rows.Next() {
		var snapshot RangeSnapshot
		var start, end int64
		var status string
		var leasedUntil sql.NullTime
		if err := rows.Scan(&start, &end, &status, &snapshot.WorkerID, &leasedUntil); err != nil {
			return nil, 0, err
		}
		snapshot.Start = uint64(start)
		snapshot.End = uint64(end)
		snapshot.Status = domain.RangeStatus(status)
		if leasedUntil.Valid {
			snapshot.LeasedUntil = leasedUntil.Time
		}
		if snapshot.Status == domain.RangeStatusLeased && !snapshot.LeasedUntil.IsZero() && time.Now().After(snapshot.LeasedUntil) {
			snapshot.Status = domain.RangeStatusPending
			snapshot.WorkerID = ""
			snapshot.LeasedUntil = time.Time{}
		}
		if snapshot.End > totalCandidates {
			totalCandidates = snapshot.End
		}
		ranges = append(ranges, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return ranges, totalCandidates, nil
}

func (s *PostgresStore) SaveJob(ctx context.Context, snapshot JobSnapshot) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO jobs (id, target_hash, charset, min_length, max_length, status, plaintext)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''))
		ON CONFLICT (id) DO UPDATE SET
			target_hash = EXCLUDED.target_hash,
			charset = EXCLUDED.charset,
			min_length = EXCLUDED.min_length,
			max_length = EXCLUDED.max_length,
			status = EXCLUDED.status,
			plaintext = EXCLUDED.plaintext,
			updated_at = now()
	`, snapshot.Job.ID, snapshot.Job.TargetHash, snapshot.Job.Charset, snapshot.Job.MinLength, snapshot.Job.MaxLength, snapshot.Job.Status, snapshot.Plaintext)
	if err != nil {
		return err
	}

	for _, searchRange := range snapshot.Ranges {
		start, err := uint64ToInt64(searchRange.Start)
		if err != nil {
			return err
		}
		end, err := uint64ToInt64(searchRange.End)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO search_ranges (job_id, range_start, range_end, status, worker_id, leased_until)
			VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6)
			ON CONFLICT (job_id, range_start, range_end) DO UPDATE SET
				status = EXCLUDED.status,
				worker_id = EXCLUDED.worker_id,
				leased_until = EXCLUDED.leased_until,
				updated_at = now()
		`, snapshot.Job.ID, start, end, searchRange.Status, searchRange.WorkerID, nullableTime(searchRange.LeasedUntil))
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *PostgresStore) UpdateRange(ctx context.Context, jobID string, searchRange RangeSnapshot) error {
	start, err := uint64ToInt64(searchRange.Start)
	if err != nil {
		return err
	}
	end, err := uint64ToInt64(searchRange.End)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE search_ranges
		SET status = $4, worker_id = NULLIF($5, ''), leased_until = $6, updated_at = now()
		WHERE job_id = $1 AND range_start = $2 AND range_end = $3
	`, jobID, start, end, searchRange.Status, searchRange.WorkerID, nullableTime(searchRange.LeasedUntil))
	return err
}

func (s *PostgresStore) UpdateJob(ctx context.Context, jobID string, status domain.JobStatus, plaintext string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE jobs
		SET status = $2, plaintext = NULLIF($3, ''), updated_at = now()
		WHERE id = $1
	`, jobID, status, plaintext)
	return err
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func uint64ToInt64(value uint64) (int64, error) {
	if value > math.MaxInt64 {
		return 0, fmt.Errorf("value %d exceeds PostgreSQL BIGINT limit", value)
	}
	return int64(value), nil
}

func rollback(tx *sql.Tx) {
	_ = tx.Rollback()
}
