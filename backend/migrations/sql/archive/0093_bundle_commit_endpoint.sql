UPDATE resource_schemas
SET schema = jsonb_set(
    schema,
    '{properties,endpoint}',
    '{"type":"string","title":"仓库端点（分支:commit ID）","readOnly":true}'::jsonb,
    true
)
WHERE kind = 'Repository';
