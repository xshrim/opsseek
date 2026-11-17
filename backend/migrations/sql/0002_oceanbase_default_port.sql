UPDATE resource_schemas
   SET schema = jsonb_set(schema, '{properties,port,default}', '2883'::jsonb)
 WHERE kind = 'OceanBase'
   AND schema #> '{properties,port,default}' = '2881'::jsonb;
