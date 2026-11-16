WITH parsed AS (
    SELECT id,
           config->>'url' AS remote,
           (regexp_match(config->>'endpoint', '^.* \((.*):([[:xdigit:]]{40}|[[:xdigit:]]{64})\)$'))[1] AS branch,
           (regexp_match(config->>'endpoint', '^.* \((.*):([[:xdigit:]]{40}|[[:xdigit:]]{64})\)$'))[2] AS commit_id
    FROM resources
    WHERE kind = 'Repository' AND lower(subtype) = 'git'
)
UPDATE resources AS resource
SET config = jsonb_set(
    resource.config,
    '{endpoint}',
    to_jsonb(parsed.remote || ' [' || parsed.branch || '·' || parsed.commit_id || ']')
)
FROM parsed
WHERE resource.id = parsed.id AND parsed.branch IS NOT NULL;

WITH parsed AS (
    SELECT id,
           (regexp_match(config->>'endpoint', '^([^:]+):([[:xdigit:]]{40}|[[:xdigit:]]{64})$'))[1] AS branch,
           (regexp_match(config->>'endpoint', '^([^:]+):([[:xdigit:]]{40}|[[:xdigit:]]{64})$'))[2] AS commit_id
    FROM resources
    WHERE kind = 'Repository' AND lower(subtype) = 'bundle'
)
UPDATE resources AS resource
SET config = jsonb_set(
    resource.config,
    '{endpoint}',
    to_jsonb(resource.id::text || ' [' || parsed.branch || '·' || parsed.commit_id || ']')
)
FROM parsed
WHERE resource.id = parsed.id AND parsed.branch IS NOT NULL;
