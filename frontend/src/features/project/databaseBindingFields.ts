export function databaseBindingFields(kind: string, mode = '') {
  const username = {
    key: 'username',
    label: '应用数据库用户名（可选）',
    placeholder: '应用实际使用的账号，非诊断登录凭据'
  };
  const database = {
    key: 'database',
    label: '数据库名（可选）',
    placeholder: '目标业务库；使用资源诊断凭据连接，留空沿用资源配置'
  };
  const schema = {
    key: 'schema',
    label: 'Schema（可选）',
    placeholder: '例如 public'
  };
  const service = {
    key: 'service_name',
    label: '服务名（可选）',
    placeholder: '留空沿用资源配置'
  };
  const owner = {
    key: 'schema',
    label: 'Schema / 对象所有者（可选）',
    placeholder: '例如 ORDERS'
  };
  if (kind === 'MySQL') return [database, username];
  if (kind === 'Oracle')
    return [
      service,
      {
        key: 'sid',
        label: 'SID（可选）',
        placeholder: '与服务名二选一；留空沿用资源配置'
      },
      owner,
      username
    ];
  if (kind === 'OceanBase') {
    if (mode.toLowerCase() === 'oracle') return [service, owner, username];
    if (mode.toLowerCase() === 'mysql') return [database, username];
    return [database, owner, username];
  }
  return [database, schema, username];
}
