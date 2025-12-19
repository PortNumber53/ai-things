package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ai-things/manager-go/internal/utils"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

type Content struct {
	ID        int64
	Title     string
	Status    *string
	Type      *string
	Sentences []byte
	Count     int
	Meta      []byte
	Archive   []byte
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewStore(ctx context.Context, connString string) (*Store, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) GetContentByID(ctx context.Context, id int64) (Content, error) {
	utils.Logf("db: get content id=%d", id)
	row := s.pool.QueryRow(ctx, `
		SELECT id, title, status, type, sentences, count, meta, archive, created_at, updated_at
		FROM contents
		WHERE id = $1
	`, id)

	var c Content
	err := row.Scan(
		&c.ID,
		&c.Title,
		&c.Status,
		&c.Type,
		&c.Sentences,
		&c.Count,
		&c.Meta,
		&c.Archive,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	return c, err
}

func (s *Store) FindFirstContent(ctx context.Context, where string, args ...any) (Content, error) {
	query := `
		SELECT id, title, status, type, sentences, count, meta, archive, created_at, updated_at
		FROM contents
		` + where + `
		ORDER BY id
		LIMIT 1
	`
	utils.Logf("db: find first query=%s args=%v", strings.TrimSpace(query), args)
	row := s.pool.QueryRow(ctx, query, args...)
	var c Content
	err := row.Scan(
		&c.ID,
		&c.Title,
		&c.Status,
		&c.Type,
		&c.Sentences,
		&c.Count,
		&c.Meta,
		&c.Archive,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	return c, err
}

func (s *Store) CountContent(ctx context.Context, where string, args ...any) (int, error) {
	query := `SELECT COUNT(*) FROM contents ` + where
	utils.Logf("db: count query=%s args=%v", strings.TrimSpace(query), args)
	row := s.pool.QueryRow(ctx, query, args...)
	var count int
	return count, row.Scan(&count)
}

func (s *Store) UpdateContentMetaStatus(ctx context.Context, id int64, status string, meta map[string]any) error {
	utils.Logf("db: update meta+status id=%d status=%s", id, status)
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE contents
		SET status = $1,
			meta = $2,
			updated_at = NOW()
		WHERE id = $3
	`, status, metaJSON, id)
	return err
}

func (s *Store) UpdateContentMeta(ctx context.Context, id int64, meta map[string]any) error {
	utils.Logf("db: update meta id=%d", id)
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE contents
		SET meta = $1,
			updated_at = NOW()
		WHERE id = $2
	`, metaJSON, id)
	return err
}

func (s *Store) UpdateContentStatus(ctx context.Context, id int64, status string) error {
	utils.Logf("db: update status id=%d status=%s", id, status)
	_, err := s.pool.Exec(ctx, `
		UPDATE contents
		SET status = $1,
			updated_at = NOW()
		WHERE id = $2
	`, status, id)
	return err
}

func StatusTrueCondition(flags []string) string {
	conds := make([]string, 0, len(flags))
	for _, flag := range flags {
		conds = append(conds, fmt.Sprintf("meta->'status'->>'%s' = 'true'", flag))
	}
	return strings.Join(conds, " AND ")
}

func StatusNotTrueCondition(flags []string) string {
	conds := make([]string, 0, len(flags))
	for _, flag := range flags {
		conds = append(conds, fmt.Sprintf("(meta->'status'->>'%s' IS NULL OR meta->'status'->>'%s' <> 'true')", flag, flag))
	}
	return strings.Join(conds, " AND ")
}

func StatusFalseCondition(flags []string) string {
	conds := make([]string, 0, len(flags))
	for _, flag := range flags {
		conds = append(conds, fmt.Sprintf("meta->'status'->>'%s' = 'false'", flag))
	}
	return strings.Join(conds, " AND ")
}

func MetaKeyMissingCondition(keys []string) string {
	conds := make([]string, 0, len(keys))
	for _, key := range keys {
		conds = append(conds, fmt.Sprintf("NOT (meta ? '%s')", key))
	}
	return strings.Join(conds, " AND ")
}
