-- Converted from Laravel migration: manager/database/migrations/2024_10_21_211557_create_subscriptions_table.php

CREATE TABLE IF NOT EXISTS public.subscriptions (
  id BIGSERIAL PRIMARY KEY,
  feed_url VARCHAR NOT NULL,
  title VARCHAR NULL,
  description VARCHAR NULL,
  site_url VARCHAR NULL,
  last_fetched_at TIMESTAMP NULL,
  last_build_date TIMESTAMP NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMP NULL,
  updated_at TIMESTAMP NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS subscriptions_feed_url_unique ON public.subscriptions (feed_url);


