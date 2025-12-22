-- Converted from Laravel migration: manager/database/migrations/2024_11_26_051512_add_processed_at_to_collections_table.php

ALTER TABLE public.collections
  ADD COLUMN IF NOT EXISTS processed_at TIMESTAMP NULL;


