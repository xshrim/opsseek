INSERT INTO resource_schemas (
    id, kind, version, schema, status, display_name, description, icon
) VALUES (
    'b3d2e3a1-7f4e-4d6b-9a2c-1f8e6b5c4d30',
    'MongoDB',
    1,
    '{
      "$schema": "https://json-schema.org/draft/2020-12/schema",
      "type": "object",
      "required": ["host"],
      "properties": {
        "host": {"type": "string", "title": "主机", "minLength": 1},
        "port": {"type": "integer", "title": "端口", "default": 27017, "minimum": 1, "maximum": 65535},
        "database": {"type": "string", "title": "数据库"},
        "auth_source": {"type": "string", "title": "认证数据库", "default": "admin"},
        "username": {"type": "string", "title": "用户名", "sensitive": true},
        "password": {"type": "string", "title": "密码", "sensitive": true},
        "timeout_seconds": {"type": "integer", "title": "超时时间（秒）", "default": 10, "minimum": 1, "maximum": 300}
      },
      "additionalProperties": false
    }'::jsonb,
    'active',
    'MongoDB',
    'MongoDB 文档数据库资源。',
    'iconify:simple-icons:mongodb'
)
ON CONFLICT (kind, version) DO NOTHING;

UPDATE resource_schemas
   SET display_name = 'Elasticsearch',
       description = 'Elasticsearch 文档数据库资源，支持集群、索引和分片诊断。'
 WHERE kind = 'Elasticsearch' AND version = 1;
