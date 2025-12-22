-- Slack thread sessions: when a thread is "activated" by an app_mention, continue
-- responding to subsequent message replies in that thread without requiring another @mention.

CREATE TABLE IF NOT EXISTS slack_thread_sessions (
  team_id TEXT NOT NULL,
  channel_id TEXT NOT NULL,
  thread_ts TEXT NOT NULL,
  activated_by_user_id TEXT,
  activated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (team_id, channel_id, thread_ts)
);

CREATE INDEX IF NOT EXISTS idx_slack_thread_sessions_expires_at
  ON slack_thread_sessions(expires_at);


