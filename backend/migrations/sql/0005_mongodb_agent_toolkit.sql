UPDATE resource_schemas
   SET schema = schema - 'required',
       description = 'MongoDB 文档数据库资源，支持 Direct 和 Agent 只读诊断工具。'
 WHERE kind = 'MongoDB' AND version = 1;
