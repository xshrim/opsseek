ALTER TABLE inspection_runs DROP COLUMN IF EXISTS expert_group_snapshot;
ALTER TABLE diagnosis_sessions DROP COLUMN IF EXISTS expert_group_snapshot;
ALTER TABLE diagnosis_sessions DROP COLUMN IF EXISTS expert_group_id;
ALTER TABLE inspection_policies DROP COLUMN IF EXISTS expert_group_id;
DROP TABLE IF EXISTS expert_group_bindings;
DROP TABLE IF EXISTS expert_group_members;
DROP TABLE IF EXISTS expert_groups;
