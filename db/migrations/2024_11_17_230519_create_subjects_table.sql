-- Converted from Laravel migration: manager/database/migrations/2024_11_17_230519_create_subjects_table.php

CREATE TABLE IF NOT EXISTS public.subjects (
  id BIGSERIAL PRIMARY KEY,
  subject VARCHAR NOT NULL,
  keywords TEXT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  podcasts_count INTEGER NOT NULL DEFAULT 0,
  last_used_at TIMESTAMP NULL,
  created_at TIMESTAMP NULL,
  updated_at TIMESTAMP NULL
);


