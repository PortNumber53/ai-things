-- Slack App installations (OAuth v2 bot token per workspace).
-- Apply this to the same Postgres DB used by manager-go.

CREATE TABLE IF NOT EXISTS slack_installations (
  team_id TEXT PRIMARY KEY,
  team_name TEXT,
  bot_user_id TEXT,
  bot_token TEXT NOT NULL,
  scope TEXT,
  installed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_slack_installations_updated_at
  ON slack_installations(updated_at DESC);


