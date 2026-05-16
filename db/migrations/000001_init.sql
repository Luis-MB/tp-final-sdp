CREATE TABLE IF NOT EXISTS jobs (
  id TEXT PRIMARY KEY,
  target_hash TEXT NOT NULL,
  charset TEXT NOT NULL,
  min_length INTEGER NOT NULL,
  max_length INTEGER NOT NULL,
  status TEXT NOT NULL,
  plaintext TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS search_ranges (
  id BIGSERIAL PRIMARY KEY,
  job_id TEXT NOT NULL REFERENCES jobs(id),
  range_start BIGINT NOT NULL,
  range_end BIGINT NOT NULL,
  status TEXT NOT NULL,
  worker_id TEXT,
  leased_until TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_search_ranges_job_status
  ON search_ranges(job_id, status);

