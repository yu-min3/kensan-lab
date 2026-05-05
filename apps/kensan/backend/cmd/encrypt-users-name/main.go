// Command encrypt-users-name is a one-shot batch tool that backfills
// users.name_enc and users.name_hash for every existing row, given the legacy
// users.name plaintext column. Designed for the Stage 6 (Vault Transit)
// migration: run *after* applying 008_users_name_transit.sql and *before*
// applying 009_drop_users_name.sql.
//
// Idempotency
//   - Rows whose name_enc is already non-NULL are skipped by default.
//     Pass -force to re-encrypt (e.g., after a Vault key rotation when you
//     want to upgrade ciphertexts to the latest key version).
//
// Usage examples
//
//	# Production (against in-cluster Postgres + Vault)
//	DB_HOST=postgresql.kensan-data.svc.cluster.local \
//	DB_USER=$(...) DB_PASSWORD=$(...) DB_NAME=kensan \
//	VAULT_ADDR=http://vault.vault.svc.cluster.local:8200 \
//	VAULT_AUTH_ROLE=kensan-users-transit \
//	VAULT_TRANSIT_KEY=users-name \
//	  go run ./cmd/encrypt-users-name -dry-run
//
//	# Local dev (NoOpEncryptor — name_enc gets "noop:<plain>" prefix)
//	DB_HOST=localhost DB_USER=kensan DB_PASSWORD=kensan DB_NAME=kensan \
//	  go run ./cmd/encrypt-users-name
//
// Exit codes
//
//	0 — success (or dry-run completed)
//	1 — fatal error
//	2 — partial failure (one or more rows failed encrypt; tool prints summary)
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kensan/backend/shared/config"
	"github.com/kensan/backend/shared/database"
	"github.com/kensan/backend/shared/vault"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "log what would change without writing")
	force := flag.Bool("force", false, "re-encrypt rows even if name_enc is already populated")
	batchSize := flag.Int("batch", 100, "rows per batch (just for logging cadence)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := run(*dryRun, *force, *batchSize); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(dryRun, force bool, batchSize int) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg := config.Load()

	pool, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()

	vcfg := vault.LoadConfigFromEnv()
	if vcfg.KeyName == "" {
		vcfg.KeyName = "users-name"
	}
	encryptor, err := buildEncryptor(ctx, vcfg)
	if err != nil {
		return fmt.Errorf("build encryptor: %w", err)
	}
	if c, ok := encryptor.(*vault.Client); ok {
		defer c.Close()
	}

	slog.Info("starting backfill",
		"dry_run", dryRun,
		"force", force,
		"vault_addr", vcfg.Address,
		"vault_key", vcfg.KeyName)

	// Stream rows so very large tables don't OOM. Order by id for deterministic
	// resume behavior across runs.
	whereClause := "WHERE name IS NOT NULL"
	if !force {
		whereClause += " AND (name_enc IS NULL OR name_hash IS NULL)"
	}
	query := "SELECT id, name FROM users " + whereClause + " ORDER BY id"

	return processRows(ctx, pool, encryptor, query, dryRun, batchSize)
}

func processRows(
	ctx context.Context,
	pool *pgxpool.Pool,
	encryptor vault.Encryptor,
	query string,
	dryRun bool,
	batchSize int,
) error {
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("select users: %w", err)
	}
	defer rows.Close()

	var (
		processed int
		updated   int
		skipped   int
		failed    int
		started   = time.Now()
	)

	for rows.Next() {
		var (
			id   string
			name string
		)
		if err := rows.Scan(&id, &name); err != nil {
			slog.Error("scan failed", "error", err)
			failed++
			continue
		}
		processed++

		if name == "" {
			slog.Warn("empty name, skipping (cannot encrypt empty plaintext)", "id", id)
			skipped++
			continue
		}

		ct, err := encryptor.Encrypt(ctx, []byte(name))
		if err != nil {
			slog.Error("encrypt failed", "id", id, "error", err)
			failed++
			continue
		}
		hm, err := encryptor.HMAC(ctx, []byte(name))
		if err != nil {
			slog.Error("hmac failed", "id", id, "error", err)
			failed++
			continue
		}

		if dryRun {
			slog.Info("dry-run: would update",
				"id", id,
				"ct_prefix", safePrefix(ct, 16),
				"hm_prefix", safePrefix(hm, 16))
			updated++
		} else {
			tag, err := pool.Exec(ctx,
				`UPDATE users SET name_enc = $1, name_hash = $2 WHERE id = $3`,
				[]byte(ct), []byte(hm), id)
			if err != nil {
				slog.Error("update failed", "id", id, "error", err)
				failed++
				continue
			}
			if tag.RowsAffected() != 1 {
				slog.Warn("unexpected rows affected", "id", id, "affected", tag.RowsAffected())
			}
			updated++
		}

		if processed%batchSize == 0 {
			slog.Info("progress",
				"processed", processed,
				"updated", updated,
				"failed", failed,
				"elapsed", time.Since(started).Round(time.Millisecond))
		}
	}

	if err := rows.Err(); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("rows iter: %w", err)
	}

	slog.Info("done",
		"processed", processed,
		"updated", updated,
		"skipped", skipped,
		"failed", failed,
		"elapsed", time.Since(started).Round(time.Millisecond),
		"dry_run", dryRun)

	if failed > 0 {
		os.Exit(2)
	}
	return nil
}

func buildEncryptor(ctx context.Context, cfg vault.Config) (vault.Encryptor, error) {
	cli, err := vault.NewClient(ctx, cfg)
	if err == nil {
		slog.Info("Vault Transit encryptor initialized",
			"address", cfg.Address,
			"role", cfg.Role,
			"mount", cfg.MountPath,
			"key", cfg.KeyName)
		return cli, nil
	}
	if errors.Is(err, vault.ErrNotConfigured) {
		slog.Warn("VAULT_ADDR unset — using NoOpEncryptor (DO NOT use in production)")
		return vault.NoOpEncryptor{}, nil
	}
	return nil, err
}

// safePrefix returns up to n bytes of s, used to log ciphertext prefix without
// dumping full Vault payloads. We never log plaintext names.
func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
