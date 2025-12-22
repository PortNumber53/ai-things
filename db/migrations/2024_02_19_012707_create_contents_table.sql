-- Converted from Laravel migration: manager/database/migrations/2024_02_19_012707_create_contents_table.php

CREATE TABLE IF NOT EXISTS public.contents (
  id BIGSERIAL PRIMARY KEY,
  title VARCHAR NOT NULL,
  status VARCHAR NULL,
  type VARCHAR NULL,
  sentences JSONB NOT NULL,
  count INTEGER NOT NULL,
  meta JSONB NULL,
  created_at TIMESTAMP NULL,
  updated_at TIMESTAMP NULL
);


