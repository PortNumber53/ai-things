package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ai-things/manager-go/internal/config"
	"ai-things/manager-go/internal/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func runMigrate(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	dir := fs.String("dir", defaultMigrationsDir(cfg), "Directory containing *.sql migrations")
	dryRun := fs.Bool("dry-run", false, "List pending migrations without applying")
	verbose := fs.Bool("verbose", utils.Verbose, "Verbose logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	utils.ConfigureLogging(*verbose)

	action := "up"
	if len(fs.Args()) > 0 {
		action = strings.TrimSpace(fs.Args()[0])
	}
	if action == "" {
		action = "up"
	}
	if action != "up" {
		return fmt.Errorf("unsupported migrate action %q (supported: up)", action)
	}

	pool, err := pgxpool.New(ctx, cfg.DBConnString())
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		return err
	}

	files, err := listSQLFiles(*dir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no .sql files found in %s", *dir)
	}

	pending := []string{}
	for _, path := range files {
		name := filepath.Base(path)
		applied, err := isApplied(ctx, pool, name)
		if err != nil {
			return err
		}
		if !applied {
			pending = append(pending, path)
		}
	}

	if *dryRun {
		for _, p := range pending {
			fmt.Println(filepath.Base(p))
		}
		return nil
	}

	appliedCount := 0
	for _, path := range pending {
		name := filepath.Base(path)
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sqlText := strings.TrimSpace(string(sqlBytes))
		if sqlText == "" {
			continue
		}
		start := time.Now()
		utils.Info("migrate apply", "migration", name)

		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		_, execErr := tx.Exec(ctx, sqlText)
		if execErr == nil {
			_, execErr = tx.Exec(ctx, `INSERT INTO schema_migrations (filename, applied_at) VALUES ($1, NOW())`, name)
		}
		if execErr != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s failed: %w", name, execErr)
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		appliedCount++
		utils.Info("migrate applied", "migration", name, "dur", time.Since(start).Truncate(time.Millisecond).String())
	}

	fmt.Printf("Applied %d migration(s)\n", appliedCount)
	return nil
}

func ensureMigrationsTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func isApplied(ctx context.Context, pool *pgxpool.Pool, filename string) (bool, error) {
	var out string
	err := pool.QueryRow(ctx, `SELECT filename FROM schema_migrations WHERE filename = $1`, filename).Scan(&out)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return out != "", nil
}

func listSQLFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".sql") {
			out = append(out, filepath.Join(dir, name))
		}
	}
	sort.Strings(out)
	return out, nil
}
