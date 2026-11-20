-- I008 C1-4 (R001 FR-008): Engine catalog runtime columns. Additive only;
-- existing rows keep the ADK default. The per-execution runtime snapshot
-- (kind + version) travels in the execution event payload: the platform
-- Runtime records it when it opens execution.started, so no execution table
-- column is required.
ALTER TABLE engines ADD COLUMN runtime_kind text DEFAULT 'adk'::text NOT NULL;
ALTER TABLE engines ADD COLUMN runtime_version text;
ALTER TABLE engines ADD COLUMN experimental boolean DEFAULT false NOT NULL;
ALTER TABLE engines ADD CONSTRAINT engines_runtime_kind_check CHECK ((runtime_kind = ANY (ARRAY['adk'::text, 'pi'::text])));
