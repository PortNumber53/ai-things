-- Slack workspace settings (per team_id).
-- Currently used to store the default channel for Slack-driven image generation.

CREATE TABLE IF NOT EXISTS slack_team_settings (
  team_id TEXT PRIMARY KEY,
  image_channel_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_slack_team_settings_updated_at
  ON slack_team_settings(updated_at DESC);


