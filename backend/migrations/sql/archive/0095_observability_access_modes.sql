INSERT INTO resource_schemas (kind, version, schema, status, display_name, description, icon)
VALUES (
    'Skywalking',
    1,
    '{"type":"object","$schema":"https://json-schema.org/draft/2020-12/schema","additionalProperties":true}'::jsonb,
    'active',
    'Skywalking',
    'Skywalking distributed tracing resource.',
    'lucide:Route'
);

UPDATE resources
   SET subtype = 'Direct'
 WHERE kind IN ('Prometheus', 'Loki', 'Tempo', 'Jaeger', 'Skywalking', 'Alertmanager')
   AND lower(COALESCE(subtype, '')) NOT IN ('direct', 'agent');
