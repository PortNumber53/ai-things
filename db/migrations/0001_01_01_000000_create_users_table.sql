-- Converted from Laravel migration: manager/database/migrations/0001_01_01_000000_create_users_table.php

CREATE TABLE IF NOT EXISTS public.users (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR NOT NULL,
  email VARCHAR NOT NULL,
  email_verified_at TIMESTAMP NULL,
  password VARCHAR NOT NULL,
  remember_token VARCHAR(100) NULL,
  created_at TIMESTAMP NULL,
  updated_at TIMESTAMP NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS users_email_unique ON public.users (email);

CREATE TABLE IF NOT EXISTS public.password_reset_tokens (
  email VARCHAR PRIMARY KEY,
  token VARCHAR NOT NULL,
  created_at TIMESTAMP NULL
);

CREATE TABLE IF NOT EXISTS public.sessions (
  id VARCHAR PRIMARY KEY,
  user_id BIGINT NULL,
  ip_address VARCHAR(45) NULL,
  user_agent TEXT NULL,
  payload TEXT NOT NULL,
  last_activity INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS sessions_user_id_index ON public.sessions (user_id);
CREATE INDEX IF NOT EXISTS sessions_last_activity_index ON public.sessions (last_activity);

