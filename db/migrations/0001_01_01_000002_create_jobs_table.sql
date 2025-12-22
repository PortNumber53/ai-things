-- Converted from Laravel migration: manager/database/migrations/0001_01_01_000002_create_jobs_table.php

CREATE TABLE IF NOT EXISTS public.jobs (
  id BIGSERIAL PRIMARY KEY,
  queue VARCHAR NOT NULL,
  payload TEXT NOT NULL,
  attempts SMALLINT NOT NULL,
  reserved_at INTEGER NULL,
  available_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS jobs_queue_index ON public.jobs (queue);

CREATE TABLE IF NOT EXISTS public.job_batches (
  id VARCHAR PRIMARY KEY,
  name VARCHAR NOT NULL,
  total_jobs INTEGER NOT NULL,
  pending_jobs INTEGER NOT NULL,
  failed_jobs INTEGER NOT NULL,
  failed_job_ids TEXT NOT NULL,
  options TEXT NULL,
  cancelled_at INTEGER NULL,
  created_at INTEGER NOT NULL,
  finished_at INTEGER NULL
);

CREATE TABLE IF NOT EXISTS public.failed_jobs (
  id BIGSERIAL PRIMARY KEY,
  uuid VARCHAR NOT NULL,
  connection TEXT NOT NULL,
  queue TEXT NOT NULL,
  payload TEXT NOT NULL,
  exception TEXT NOT NULL,
  failed_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS failed_jobs_uuid_unique ON public.failed_jobs (uuid);


