-- Converted from Laravel migration: manager/database/migrations/2024_11_19_185351_add_archive_column_to_contents.php

ALTER TABLE public.contents
  ADD COLUMN IF NOT EXISTS archive JSONB NULL;


