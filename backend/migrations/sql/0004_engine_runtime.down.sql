-- I008 C1-4 rollback: drop the runtime columns added by 0004_engine_runtime.
ALTER TABLE engines DROP CONSTRAINT IF EXISTS engines_runtime_kind_check;
ALTER TABLE engines DROP COLUMN IF EXISTS experimental;
ALTER TABLE engines DROP COLUMN IF EXISTS runtime_version;
ALTER TABLE engines DROP COLUMN IF EXISTS runtime_kind;
