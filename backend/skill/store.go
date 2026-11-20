package skill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store interface {
	CreateVersion(context.Context, CreateVersionInput) (Version, error)
	ImportVersion(context.Context, CreateVersionInput) (Version, bool, error)
	FindSkillByIdentifier(context.Context, string, string) (Skill, error)
	CreateSkill(context.Context, Skill) (Skill, error)
	GetVersion(context.Context, string) (Version, error)
	ListVersions(context.Context, string) ([]Version, error)
	PublishVersion(context.Context, string, string) (Version, error)
	DisableVersion(context.Context, string, string) (Version, error)
	SetDefault(context.Context, Default, string) (Default, error)
	ResolveDefault(context.Context, string) (Default, error)
}

type SkillReader interface {
	GetSkill(context.Context, string) (Skill, error)
	ListSkills(context.Context, string) ([]Skill, error)
}

type store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) Store { return &store{pool: pool} }

func NewCatalog(pool *pgxpool.Pool) SkillReader { return &store{pool: pool} }

func (s *store) GetSkill(ctx context.Context, id string) (Skill, error) {
	var item Skill
	var tags []byte
	err := s.pool.QueryRow(ctx, `SELECT id::text, scope_id::text, name, identifier, category, tags, maintainer, status, created_at, updated_at FROM skills WHERE id = $1::uuid AND deleted_at IS NULL`, id).Scan(&item.ID, &item.ScopeID, &item.Name, &item.Identifier, &item.Category, &tags, &item.Maintainer, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Skill{}, ErrNotFound
	}
	if err != nil {
		return Skill{}, fmt.Errorf("get skill: %w", err)
	}
	if err := json.Unmarshal(tags, &item.Tags); err != nil {
		return Skill{}, fmt.Errorf("decode skill tags: %w", err)
	}
	return item, nil
}

func (s *store) ListSkills(ctx context.Context, scopeID string) ([]Skill, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, scope_id::text, name, identifier, category, tags, maintainer, status, created_at, updated_at FROM skills WHERE scope_id = $1::uuid AND deleted_at IS NULL ORDER BY name`, scopeID)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	defer rows.Close()
	items := make([]Skill, 0)
	for rows.Next() {
		var item Skill
		var tags []byte
		if err := rows.Scan(&item.ID, &item.ScopeID, &item.Name, &item.Identifier, &item.Category, &tags, &item.Maintainer, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(tags, &item.Tags); err != nil {
			return nil, fmt.Errorf("decode skill tags: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) CreateVersion(ctx context.Context, input CreateVersionInput) (Version, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Version{}, fmt.Errorf("begin Skill version transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, input.SkillID); err != nil {
		return Version{}, fmt.Errorf("lock Skill versions: %w", err)
	}
	item, err := insertVersion(ctx, tx, input)
	if err != nil {
		return Version{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Version{}, fmt.Errorf("commit Skill version: %w", err)
	}
	return item, nil
}

func (s *store) GetVersion(ctx context.Context, id string) (Version, error) {
	return getVersion(ctx, s.pool, id)
}

// ImportVersion creates a version under the Skill's advisory lock and enforces
// the import invariants inside the same transaction: same source_digest
// returns the already imported version (created=false, idempotent) and an
// identical manifest version string is rejected outright (R001 §3.2.3/§3.3).
func (s *store) ImportVersion(ctx context.Context, input CreateVersionInput) (Version, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Version{}, false, fmt.Errorf("begin Skill import transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, input.SkillID); err != nil {
		return Version{}, false, fmt.Errorf("lock Skill versions: %w", err)
	}
	var existingID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM skill_versions WHERE skill_id = $1::uuid AND source_digest = $2 LIMIT 1`, input.SkillID, input.SourceDigest).Scan(&existingID)
	if err == nil {
		item, getErr := getVersion(ctx, tx, existingID)
		if getErr != nil {
			return Version{}, false, getErr
		}
		if err := tx.Commit(ctx); err != nil {
			return Version{}, false, fmt.Errorf("commit Skill import lookup: %w", err)
		}
		return item, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Version{}, false, fmt.Errorf("lookup imported Skill digest: %w", err)
	}
	if input.Manifest.Version != "" {
		var dupID string
		err = tx.QueryRow(ctx, `SELECT id::text FROM skill_versions WHERE skill_id = $1::uuid AND manifest ->> 'version' = $2 LIMIT 1`, input.SkillID, input.Manifest.Version).Scan(&dupID)
		if err == nil {
			return Version{}, false, ErrConflict
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return Version{}, false, fmt.Errorf("lookup imported Skill version: %w", err)
		}
	}
	item, err := insertVersion(ctx, tx, input)
	if err != nil {
		return Version{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Version{}, false, fmt.Errorf("commit Skill import: %w", err)
	}
	return item, true, nil
}

func (s *store) FindSkillByIdentifier(ctx context.Context, scopeID, identifier string) (Skill, error) {
	var item Skill
	var tags []byte
	err := s.pool.QueryRow(ctx, `SELECT id::text, scope_id::text, name, identifier, category, tags, maintainer, status, created_at, updated_at FROM skills WHERE scope_id = $1::uuid AND identifier = $2 AND deleted_at IS NULL`, scopeID, identifier).Scan(&item.ID, &item.ScopeID, &item.Name, &item.Identifier, &item.Category, &tags, &item.Maintainer, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Skill{}, ErrNotFound
	}
	if err != nil {
		return Skill{}, fmt.Errorf("find skill by identifier: %w", err)
	}
	if err := json.Unmarshal(tags, &item.Tags); err != nil {
		return Skill{}, fmt.Errorf("decode skill tags: %w", err)
	}
	return item, nil
}

func (s *store) CreateSkill(ctx context.Context, item Skill) (Skill, error) {
	tags, _ := json.Marshal(item.Tags)
	err := s.pool.QueryRow(ctx, `
		INSERT INTO skills (id, scope_id, name, identifier, category, tags, maintainer, status)
		VALUES (gen_random_uuid(), $1::uuid, $2, $3, $4, $5, $6, 'active')
		RETURNING id::text, created_at, updated_at`,
		item.ScopeID, item.Name, item.Identifier, item.Category, tags, item.Maintainer).Scan(&item.ID, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return Skill{}, mapStoreError(err)
	}
	item.Status = "active"
	return item, nil
}

func insertVersion(ctx context.Context, tx pgx.Tx, input CreateVersionInput) (Version, error) {
	manifest, _ := json.Marshal(input.Manifest)
	tools, _ := json.Marshal(input.Tools)
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO skill_versions (skill_id, version, manifest, input_schema, output_schema, tools, risk_level, created_by, source_type, source_digest)
		SELECT $1::uuid, COALESCE(max(version), 0) + 1, $2, $3, $4, $5, $6, NULLIF($7, '')::uuid, COALESCE(NULLIF($8, ''), 'manual'), NULLIF($9, '')
		  FROM skill_versions WHERE skill_id = $1::uuid
		RETURNING id::text`, input.SkillID, manifest, input.InputSchema, input.OutputSchema, tools, input.RiskLevel, input.CreatedBy, input.SourceType, input.SourceDigest).Scan(&id)
	if err != nil {
		return Version{}, mapStoreError(err)
	}
	return getVersion(ctx, tx, id)
}

func (s *store) ListVersions(ctx context.Context, skillID string) ([]Version, error) {
	rows, err := s.pool.Query(ctx, versionSelect+` WHERE skill_id = $1::uuid ORDER BY version DESC`, skillID)
	if err != nil {
		return nil, fmt.Errorf("list Skill versions: %w", err)
	}
	defer rows.Close()
	items := make([]Version, 0)
	for rows.Next() {
		item, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) PublishVersion(ctx context.Context, skillID, versionID string) (Version, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Version{}, fmt.Errorf("begin Skill publish: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, skillID); err != nil {
		return Version{}, err
	}
	tag, err := tx.Exec(ctx, `UPDATE skill_versions SET status = 'published', published_at = now() WHERE id = $1::uuid AND skill_id = $2::uuid AND status IN ('draft', 'disabled')`, versionID, skillID)
	if err != nil {
		return Version{}, mapStoreError(err)
	}
	if tag.RowsAffected() == 0 {
		return Version{}, ErrNotFound
	}
	item, err := getVersion(ctx, tx, versionID)
	if err != nil {
		return Version{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Version{}, fmt.Errorf("commit Skill publish: %w", err)
	}
	return item, nil
}

func (s *store) DisableVersion(ctx context.Context, skillID, versionID string) (Version, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE skill_versions SET status = 'disabled' WHERE id = $1::uuid AND skill_id = $2::uuid AND status <> 'disabled'`, versionID, skillID)
	if err != nil {
		return Version{}, mapStoreError(err)
	}
	if tag.RowsAffected() == 0 {
		return Version{}, ErrNotFound
	}
	return s.GetVersion(ctx, versionID)
}

func (s *store) SetDefault(ctx context.Context, input Default, actorID string) (Default, error) {
	var item Default
	err := s.pool.QueryRow(ctx, `
		INSERT INTO skill_scope_defaults (scope_id, skill_id, skill_version_id, created_by)
		VALUES ($1::uuid, $2::uuid, $3::uuid, NULLIF($4, '')::uuid)
		ON CONFLICT (scope_id) DO UPDATE SET skill_id = EXCLUDED.skill_id, skill_version_id = EXCLUDED.skill_version_id, created_by = EXCLUDED.created_by, updated_at = now()
		RETURNING scope_id::text, skill_id::text, skill_version_id::text, created_at, updated_at`,
		input.ScopeID, input.SkillID, input.SkillVersionID, actorID).Scan(&item.ScopeID, &item.SkillID, &item.SkillVersionID, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return Default{}, mapStoreError(err)
	}
	return item, nil
}

func (s *store) ResolveDefault(ctx context.Context, scopeID string) (Default, error) {
	var item Default
	err := s.pool.QueryRow(ctx, `
		WITH RECURSIVE chain(id, parent_scope_id, depth) AS (
			SELECT id, parent_scope_id, 0 FROM scopes WHERE id = $1::uuid AND deleted_at IS NULL AND status = 'active'
			UNION ALL SELECT parent.id, parent.parent_scope_id, chain.depth + 1 FROM scopes parent JOIN chain ON parent.id = chain.parent_scope_id WHERE parent.deleted_at IS NULL AND parent.status = 'active'
		)
		SELECT defaults.scope_id::text, defaults.skill_id::text, defaults.skill_version_id::text, defaults.created_at, defaults.updated_at
		  FROM chain JOIN skill_scope_defaults defaults ON defaults.scope_id = chain.id
		  JOIN skill_versions version ON version.id = defaults.skill_version_id AND version.status = 'published'
		 ORDER BY chain.depth, defaults.updated_at DESC LIMIT 1`, scopeID).Scan(&item.ScopeID, &item.SkillID, &item.SkillVersionID, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Default{}, ErrNotFound
	}
	if err != nil {
		return Default{}, fmt.Errorf("resolve default Skill: %w", err)
	}
	return item, nil
}

const versionSelect = `SELECT id::text, skill_id::text, version, manifest, input_schema, output_schema, tools, risk_level, status, source_type, source_digest, created_by::text, created_at, published_at FROM skill_versions`

type rowScanner interface{ Scan(...any) error }

func getVersion(ctx context.Context, queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string) (Version, error) {
	return scanVersion(queryer.QueryRow(ctx, versionSelect+` WHERE id = $1::uuid`, id))
}

func scanVersion(row rowScanner) (Version, error) {
	var item Version
	var manifest, tools []byte
	if err := row.Scan(&item.ID, &item.SkillID, &item.Version, &manifest, &item.InputSchema, &item.OutputSchema, &tools, &item.RiskLevel, &item.Status, &item.SourceType, &item.SourceDigest, &item.CreatedBy, &item.CreatedAt, &item.PublishedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Version{}, ErrNotFound
		}
		return Version{}, fmt.Errorf("scan Skill version: %w", err)
	}
	if err := json.Unmarshal(manifest, &item.Manifest); err != nil {
		return Version{}, fmt.Errorf("decode Skill manifest: %w", err)
	}
	if err := json.Unmarshal(tools, &item.Tools); err != nil {
		return Version{}, fmt.Errorf("decode Skill tools: %w", err)
	}
	return item, nil
}

func mapStoreError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503", "23514", "22P02":
			return invalid("Skill references invalid or unavailable data")
		case "23505":
			return ErrConflict
		}
	}
	return fmt.Errorf("store Skill data: %w", err)
}
