package migrations

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed sql/*.sql sql/archive/0001_initial_pre_0095.sql sql/archive/008*.sql sql/archive/009*.sql
var migrationFiles embed.FS

const (
	// Keep this value stable so migrators built from different releases contend on the same lock.
	migrationAdvisoryLockID int64 = 0x4f50534b4d494752
	migrationUnlockTimeout        = 5 * time.Second
)

type migration struct {
	version  int64
	name     string
	checksum string
	sql      string
}

func Apply(ctx context.Context, pool *pgxpool.Pool) error {
	return withMigrationLock(ctx, pool, apply)
}

func apply(ctx context.Context, conn *pgxpool.Conn) error {
	migrations, err := load()
	if err != nil {
		return err
	}
	if err := consolidateLegacyHistory(ctx, conn, migrations[0]); err != nil {
		return err
	}

	applied, err := prepareMigrationHistory(ctx, conn, migrations)
	if err != nil {
		return err
	}

	for _, item := range migrations {
		if _, exists := applied[item.version]; exists {
			continue
		}

		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", item.version, err)
		}
		if _, err := tx.Exec(ctx, item.sql); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %d (%s): %w", item.version, item.name, err)
		}
		if _, err := tx.Exec(ctx,
			"INSERT INTO schema_migrations (version, name, checksum) VALUES ($1, $2, $3)",
			item.version,
			item.name,
			item.checksum,
		); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %d: %w", item.version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %d: %w", item.version, err)
		}
	}

	return nil
}

// Existing databases finish their original migration chain before adopting the baseline.
func consolidateLegacyHistory(ctx context.Context, conn *pgxpool.Conn, baseline migration) error {
	var historyExists bool
	if err := conn.QueryRow(ctx, `SELECT to_regclass('schema_migrations') IS NOT NULL`).Scan(&historyExists); err != nil || !historyExists {
		return err
	}
	var name, checksum string
	if err := conn.QueryRow(ctx, `SELECT name,checksum FROM schema_migrations WHERE version=1`).Scan(&name, &checksum); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("read baseline migration: %w", err)
	}
	if name == baseline.name && checksum == baseline.checksum {
		return nil
	}
	legacy, err := loadLegacy()
	if err != nil {
		return err
	}
	applied, err := prepareMigrationHistory(ctx, conn, legacy)
	if err != nil {
		return err
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, item := range legacy {
		if _, exists := applied[item.version]; exists {
			continue
		}
		if _, err := tx.Exec(ctx, item.sql); err != nil {
			return fmt.Errorf("complete archived migration %d: %w", item.version, err)
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM schema_migrations`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version,name,checksum) VALUES(1,$1,$2)`, baseline.name, baseline.checksum); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func prepareMigrationHistory(ctx context.Context, conn *pgxpool.Conn, migrations []migration) (map[int64]struct{}, error) {
	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version bigint PRIMARY KEY,
			name text NOT NULL,
			checksum text NOT NULL,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
		return nil, fmt.Errorf("create schema migrations table: %w", err)
	}
	embedded := make(map[int64]migration, len(migrations))
	for _, item := range migrations {
		embedded[item.version] = item
	}

	rows, err := conn.Query(ctx, "SELECT version, name, checksum FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("read migration history: %w", err)
	}
	applied := make(map[int64]struct{})
	for rows.Next() {
		var version int64
		var name, checksum string
		if err := rows.Scan(&version, &name, &checksum); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan migration history: %w", err)
		}
		item, exists := embedded[version]
		if !exists {
			rows.Close()
			return nil, fmt.Errorf("applied migration %d (%s) is not embedded in this binary", version, name)
		}
		if item.name != name {
			rows.Close()
			return nil, fmt.Errorf("applied migration %d name is %q, embedded name is %q", version, name, item.name)
		}
		if checksum != item.checksum {
			rows.Close()
			return nil, fmt.Errorf("applied migration %d (%s) checksum does not match embedded SQL", version, name)
		}
		applied[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate migration history: %w", err)
	}
	rows.Close()

	return applied, nil
}

func withMigrationLock(
	ctx context.Context,
	pool *pgxpool.Pool,
	run func(context.Context, *pgxpool.Conn) error,
) (runErr error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationAdvisoryLockID); err != nil {
		closeErr := closeMigrationConnection(conn)
		return errors.Join(fmt.Errorf("acquire migration advisory lock: %w", err), closeErr)
	}

	defer func() {
		runErr = errors.Join(runErr, releaseMigrationLock(conn))
	}()
	return run(ctx, conn)
}

func releaseMigrationLock(conn *pgxpool.Conn) error {
	ctx, cancel := context.WithTimeout(context.Background(), migrationUnlockTimeout)
	defer cancel()

	var unlocked bool
	unlockErr := conn.QueryRow(ctx,
		"SELECT pg_advisory_unlock($1)",
		migrationAdvisoryLockID,
	).Scan(&unlocked)
	if unlockErr == nil && unlocked {
		conn.Release()
		return nil
	}

	closeErr := closeMigrationConnection(conn)
	if unlockErr != nil {
		return errors.Join(fmt.Errorf("release migration advisory lock: %w", unlockErr), closeErr)
	}
	return errors.Join(errors.New("release migration advisory lock: lock was not held"), closeErr)
}

func closeMigrationConnection(conn *pgxpool.Conn) error {
	rawConn := conn.Hijack()
	ctx, cancel := context.WithTimeout(context.Background(), migrationUnlockTimeout)
	defer cancel()
	if err := rawConn.Close(ctx); err != nil {
		return fmt.Errorf("close migration connection: %w", err)
	}
	return nil
}

func load() ([]migration, error) {
	return loadDirectory("sql", false)
}

func loadLegacy() ([]migration, error) {
	return loadDirectory("sql/archive", true)
}

func loadDirectory(directory string, legacy bool) ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, directory)
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	items := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" || strings.HasSuffix(entry.Name(), ".down.sql") {
			continue
		}
		if legacy && entry.Name() != "0001_initial_pre_0095.sql" && !strings.HasPrefix(entry.Name(), "008") && !strings.HasPrefix(entry.Name(), "009") {
			continue
		}
		parts := strings.SplitN(strings.TrimSuffix(entry.Name(), ".sql"), "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}
		version, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("invalid migration version in %q", entry.Name())
		}
		if legacy && version != 1 && (version < 85 || version > 95) {
			continue
		}
		content, err := migrationFiles.ReadFile(directory + "/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}
		if len(strings.TrimSpace(string(content))) == 0 {
			return nil, errors.New("migration " + entry.Name() + " is empty")
		}
		items = append(items, migration{
			version:  version,
			name:     parts[1],
			checksum: migrationChecksum(content),
			sql:      string(content),
		})
		if legacy && version == 1 {
			items[len(items)-1].name = "initial"
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].version < items[j].version })
	for index := 1; index < len(items); index++ {
		if items[index-1].version == items[index].version {
			return nil, fmt.Errorf("duplicate migration version %d", items[index].version)
		}
	}
	return items, nil
}

func migrationChecksum(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}
