-- I010 B2-1: Skill import provenance and idempotency.
ALTER TABLE skill_versions ADD COLUMN source_type text DEFAULT 'manual'::text NOT NULL;
ALTER TABLE skill_versions ADD CONSTRAINT skill_versions_source_type_check CHECK ((source_type = ANY (ARRAY['builtin'::text, 'imported'::text, 'manual'::text])));
ALTER TABLE skill_versions ADD COLUMN source_digest text;

-- Same imported content must never create two versions of one Skill (R001 §3.2.3).
CREATE UNIQUE INDEX skill_versions_source_digest_unique ON skill_versions USING btree (skill_id, source_digest) WHERE (source_digest IS NOT NULL);

-- A1-2 follow-up: restore the 0013 diagnosis output contract on the builtin seed
-- versions of the newer domains whose output_schema shipped empty.
UPDATE skill_versions SET output_schema = '{"type":"object","required":["facts","findings","evidence","hypotheses","confidence","recommendations"],"properties":{"facts":{"type":"object"},"findings":{"type":"array"},"evidence":{"type":"array"},"hypotheses":{"type":"array"},"confidence":{"type":"number","minimum":0,"maximum":1},"recommendations":{"type":"array"}},"additionalProperties":false}'::jsonb
 WHERE id IN (
    '6bc5787c-da55-4e66-b50f-d7c8e2709f2a', -- MySQL-健康诊断 v1
    'afd50095-8671-4ddf-927f-fdccb6a9b87c', -- Kafka-健康诊断 v3
    'e35ec8d3-caf4-42d9-a118-9755896b089f', -- Elasticsearch-健康诊断 v1
    '4c75896d-72e4-44ce-a309-1c962c905db5', -- Oracle-健康诊断 v1
    '4f94c00a-a11e-41a8-9ce8-f62a2a00d732', -- RabbitMQ-健康诊断 v1
    'ec326f2a-82bb-4095-a98d-6c29ce70df95', -- MinIO-健康诊断 v1
    '9c696f02-b296-4bc1-91b2-3a2d70d66345'  -- OceanBase-健康诊断 v1
 )
   AND output_schema IN ('{}'::jsonb, '{"type": "object", "additionalProperties": true}'::jsonb);
