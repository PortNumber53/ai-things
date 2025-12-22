-- Converted from Laravel migration: manager/database/migrations/2024_11_19_231100_create_collections_table.php

CREATE TABLE IF NOT EXISTS public.collections (
  id BIGSERIAL PRIMARY KEY,
  url VARCHAR NOT NULL,
  title VARCHAR NOT NULL,
  language VARCHAR(5) NOT NULL,
  html_content TEXT NOT NULL,
  fetched_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP NULL,
  updated_at TIMESTAMP NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS collections_url_unique ON public.collections (url);


