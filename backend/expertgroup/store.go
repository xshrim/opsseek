package expertgroup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StoreImpl struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *StoreImpl { return &StoreImpl{pool: pool} }

const (
	groupFields   = `id::text, scope_id::text, identifier, name, description, status, created_at, updated_at`
	memberFields  = `id::text, group_id::text, persona_id::text, seat, scene_tags, priority, created_at, updated_at`
	bindingFields = `scope_id::text, scene_tag, group_id::text, COALESCE(application_id::text, ''), COALESCE(created_by::text, ''), created_at, updated_at`
)

type rowScanner interface{ Scan(...any) error }

func mapStoreError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return ErrConflict
		case "23503", "23514":
			return invalid("expert group references invalid or unavailable data")
		}
	}
	return err
}

func (s *StoreImpl) CreateGroup(ctx context.Context, group Group) (Group, error) {
	var id string
	err := s.pool.QueryRow(ctx, `INSERT INTO expert_groups (scope_id, identifier, name, description, status) VALUES ($1::uuid, $2, $3, $4, $5) RETURNING id::text`, group.ScopeID, group.Identifier, group.Name, group.Description, group.Status).Scan(&id)
	if err != nil {
		return Group{}, mapStoreError(err)
	}
	return s.GetGroup(ctx, id)
}

func (s *StoreImpl) GetGroup(ctx context.Context, id string) (Group, error) {
	var group Group
	err := s.pool.QueryRow(ctx, `SELECT `+groupFields+` FROM expert_groups WHERE id = $1::uuid AND deleted_at IS NULL`, id).Scan(&group.ID, &group.ScopeID, &group.Identifier, &group.Name, &group.Description, &group.Status, &group.CreatedAt, &group.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Group{}, ErrNotFound
	}
	if err != nil {
		return Group{}, fmt.Errorf("get expert group: %w", err)
	}
	return group, nil
}

func (s *StoreImpl) ListGroups(ctx context.Context, scopeID string) ([]Group, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+groupFields+` FROM expert_groups WHERE scope_id = $1::uuid AND deleted_at IS NULL ORDER BY name`, scopeID)
	if err != nil {
		return nil, fmt.Errorf("list expert groups: %w", err)
	}
	defer rows.Close()
	groups := make([]Group, 0)
	for rows.Next() {
		var group Group
		if err := rows.Scan(&group.ID, &group.ScopeID, &group.Identifier, &group.Name, &group.Description, &group.Status, &group.CreatedAt, &group.UpdatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (s *StoreImpl) UpdateGroup(ctx context.Context, id string, patch Group) (Group, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE expert_groups SET name = $2, description = $3, status = $4, updated_at = now() WHERE id = $1::uuid AND deleted_at IS NULL`, id, patch.Name, patch.Description, patch.Status)
	if err != nil {
		return Group{}, mapStoreError(err)
	}
	if tag.RowsAffected() == 0 {
		return Group{}, ErrNotFound
	}
	return s.GetGroup(ctx, id)
}

func scanMember(row rowScanner) (Member, error) {
	var member Member
	var tags []byte
	if err := row.Scan(&member.ID, &member.GroupID, &member.PersonaID, &member.Seat, &tags, &member.Priority, &member.CreatedAt, &member.UpdatedAt); err != nil {
		return Member{}, err
	}
	if len(tags) > 0 {
		_ = json.Unmarshal(tags, &member.SceneTags)
	}
	if member.SceneTags == nil {
		member.SceneTags = []string{}
	}
	return member, nil
}

func (s *StoreImpl) ListMembers(ctx context.Context, groupID string) ([]Member, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+memberFields+` FROM expert_group_members WHERE group_id = $1::uuid ORDER BY priority, created_at`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list expert group members: %w", err)
	}
	defer rows.Close()
	members := make([]Member, 0)
	for rows.Next() {
		member, err := scanMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

// ReplaceMembers performs the physical full replacement (m2) inside one
// transaction. The partial unique index on seat='lead' is the second line of
// defense behind the service-level exactly-one-lead validation (m3).
func (s *StoreImpl) ReplaceMembers(ctx context.Context, groupID string, members []Member) ([]Member, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin expert group members: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM expert_group_members WHERE group_id = $1::uuid`, groupID); err != nil {
		return nil, mapStoreError(err)
	}
	for _, member := range members {
		tags, err := json.Marshal(member.SceneTags)
		if err != nil {
			return nil, invalid("expert group member scene tags are invalid")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO expert_group_members (group_id, persona_id, seat, scene_tags, priority) VALUES ($1::uuid, $2::uuid, $3, $4::jsonb, $5)`, groupID, member.PersonaID, member.Seat, tags, member.Priority); err != nil {
			return nil, mapStoreError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit expert group members: %w", err)
	}
	return s.ListMembers(ctx, groupID)
}

func scanBinding(row rowScanner) (Binding, error) {
	var binding Binding
	if err := row.Scan(&binding.ScopeID, &binding.SceneTag, &binding.GroupID, &binding.ApplicationID, &binding.CreatedBy, &binding.CreatedAt, &binding.UpdatedAt); err != nil {
		return Binding{}, err
	}
	return binding, nil
}

func (s *StoreImpl) ListBindings(ctx context.Context, scopeID string) ([]Binding, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+bindingFields+` FROM expert_group_bindings WHERE scope_id = $1::uuid ORDER BY scene_tag, application_id NULLS FIRST`, scopeID)
	if err != nil {
		return nil, fmt.Errorf("list expert group bindings: %w", err)
	}
	defer rows.Close()
	bindings := make([]Binding, 0)
	for rows.Next() {
		binding, err := scanBinding(rows)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}
	return bindings, rows.Err()
}

func (s *StoreImpl) SetBinding(ctx context.Context, actorID string, binding Binding) (Binding, error) {
	query := `INSERT INTO expert_group_bindings (scope_id, scene_tag, group_id, application_id, created_by)
VALUES ($1::uuid, $2, $3::uuid, NULLIF($4, '')::uuid, NULLIF($5, '')::uuid)
ON CONFLICT (scope_id, scene_tag, COALESCE(application_id, '00000000-0000-0000-0000-000000000000'::uuid))
DO UPDATE SET group_id = EXCLUDED.group_id, created_by = EXCLUDED.created_by, updated_at = now()
RETURNING ` + bindingFields
	var saved Binding
	err := s.pool.QueryRow(ctx, query, binding.ScopeID, binding.SceneTag, binding.GroupID, binding.ApplicationID, actorID).Scan(&saved.ScopeID, &saved.SceneTag, &saved.GroupID, &saved.ApplicationID, &saved.CreatedBy, &saved.CreatedAt, &saved.UpdatedAt)
	if err != nil {
		return Binding{}, mapStoreError(err)
	}
	return saved, nil
}

func (s *StoreImpl) RemoveBinding(ctx context.Context, binding Binding) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM expert_group_bindings WHERE scope_id = $1::uuid AND scene_tag = $2 AND group_id = $3::uuid AND COALESCE(application_id::text, '') = COALESCE(NULLIF($4, '')::uuid::text, '')`, binding.ScopeID, binding.SceneTag, binding.GroupID, binding.ApplicationID)
	if err != nil {
		return mapStoreError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *StoreImpl) ApplicationBinding(ctx context.Context, applicationID, sceneTag string) (Binding, bool, error) {
	binding, err := scanBinding(s.pool.QueryRow(ctx, `SELECT `+bindingFields+` FROM expert_group_bindings WHERE application_id = $1::uuid AND scene_tag = $2`, applicationID, sceneTag))
	if errors.Is(err, pgx.ErrNoRows) {
		return Binding{}, false, nil
	}
	if err != nil {
		return Binding{}, false, fmt.Errorf("resolve application expert group binding: %w", err)
	}
	return binding, true, nil
}

// ScopeChainBinding resolves the nearest binding up the scope chain for one
// tag. The recursive chain query mirrors engine_scope_bindings (catalog.go);
// row_number picks the closest scope deterministically.
func (s *StoreImpl) ScopeChainBinding(ctx context.Context, scopeID, sceneTag string) (Binding, bool, error) {
	query := `
WITH RECURSIVE chain(id, depth) AS (
 SELECT id, 0 FROM scopes WHERE id = $1::uuid AND deleted_at IS NULL
 UNION ALL
 SELECT parent.id, chain.depth + 1 FROM scopes parent JOIN chain ON parent.id = (SELECT parent_scope_id FROM scopes WHERE scopes.id = chain.id)
 WHERE parent.deleted_at IS NULL
), ranked AS (
 SELECT b.scope_id, b.scene_tag, b.group_id, b.application_id, b.created_by, b.created_at, b.updated_at,
        row_number() OVER (ORDER BY chain.depth) AS rank
   FROM expert_group_bindings b JOIN chain ON chain.id = b.scope_id
  WHERE b.scene_tag = $2 AND b.application_id IS NULL
)
SELECT ` + "scope_id::text, scene_tag, group_id::text, COALESCE(application_id::text, ''), COALESCE(created_by::text, ''), created_at, updated_at" + `
  FROM ranked WHERE rank = 1`
	binding, err := scanBinding(s.pool.QueryRow(ctx, query, scopeID, sceneTag))
	if errors.Is(err, pgx.ErrNoRows) {
		return Binding{}, false, nil
	}
	if err != nil {
		return Binding{}, false, fmt.Errorf("resolve scope expert group binding: %w", err)
	}
	return binding, true, nil
}
