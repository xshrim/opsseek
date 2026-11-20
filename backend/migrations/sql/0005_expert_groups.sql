-- I009 R001: Expert groups (S1). Independent first-class domain object; groups
-- reference personas and bind to applications or scope+scene defaults.
CREATE TABLE expert_groups (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id text NOT NULL DEFAULT 'default',
    scope_id uuid NOT NULL,
    name text NOT NULL,
    identifier text NOT NULL,
    description text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE (scope_id, identifier)
);

CREATE TABLE expert_group_members (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id uuid NOT NULL REFERENCES expert_groups(id),
    persona_id uuid NOT NULL REFERENCES personas(id),
    seat text NOT NULL DEFAULT 'assistant',
    scene_tags jsonb NOT NULL DEFAULT '[]',
    priority integer NOT NULL DEFAULT 100,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (group_id, persona_id),
    CONSTRAINT expert_group_members_seat_check CHECK (seat IN ('lead', 'assistant', 'oncall'))
);

-- Exactly one lead per group is enforced by the database (m3); the service
-- additionally rejects empty groups and groups without a lead on save.
CREATE UNIQUE INDEX expert_group_members_single_lead
    ON expert_group_members (group_id) WHERE seat = 'lead';

CREATE TABLE expert_group_bindings (
    scope_id uuid NOT NULL,
    scene_tag text NOT NULL CHECK (scene_tag IN ('general', 'diagnosis', 'inspection', 'workflow')),
    group_id uuid NOT NULL REFERENCES expert_groups(id),
    application_id uuid,
    created_by uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (scope_id, scene_tag, COALESCE(application_id, '00000000-0000-0000-0000-000000000000'::uuid))
);

-- Inspection: a policy may select an expert group instead of a single persona
-- (mutually exclusive, validated by the service; Q3 ruling).
ALTER TABLE inspection_policies ADD COLUMN expert_group_id uuid;

-- Diagnosis: the effective expert group and its frozen member snapshot are
-- persisted on the session so mid-session group edits never affect a running
-- conversation (M4).
ALTER TABLE diagnosis_sessions ADD COLUMN expert_group_id uuid;
ALTER TABLE diagnosis_sessions ADD COLUMN expert_group_snapshot jsonb;

-- Inspection runs persist the same "group + members + resolved versions"
-- snapshot for post-hoc audit (Q1 ruling companion requirement).
ALTER TABLE inspection_runs ADD COLUMN expert_group_snapshot jsonb;

-- Roles that already manage personas receive the expert group permissions so
-- existing operators keep working after the upgrade.
INSERT INTO role_permissions (role_id, permission)
SELECT role_id, 'expert_group:read' FROM role_permissions WHERE permission = 'persona:read'
UNION
SELECT role_id, 'expert_group:manage' FROM role_permissions WHERE permission = 'persona:manage'
UNION
SELECT role_id, 'expert_group:use' FROM role_permissions WHERE permission = 'persona:use';
