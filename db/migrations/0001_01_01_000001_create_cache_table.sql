-- Converted from Laravel migration: manager/database/migrations/0001_01_01_000001_create_cache_table.php

CREATE TABLE IF NOT EXISTS public.cache (
  key VARCHAR PRIMARY KEY,
  value TEXT NOT NULL,
  expiration INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS public.cache_locks (
  key VARCHAR PRIMARY KEY,
  owner VARCHAR NOT NULL,
  expiration INTEGER NOT NULL
);


