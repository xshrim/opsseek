UPDATE resource_schemas
SET schema = schema #- '{properties,endpoint}'
WHERE kind = 'Repository';
