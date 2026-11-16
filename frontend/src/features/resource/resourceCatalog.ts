import type {
  ConnectorCapability,
  Resource,
  ResourceSchema
} from '../../lib/api';
export { brandNameFor } from '../../lib/resources';

export function relativeConnectionTime(value: string, now = Date.now()) {
  const timestamp = Date.parse(value);
  if (!Number.isFinite(timestamp)) return '未测试';
  const elapsed = Math.max(now - timestamp, 0);
  const minute = 60 * 1000;
  const hour = 60 * minute;
  const day = 24 * hour;
  const week = 7 * day;
  const month = 30 * day;
  const year = 365 * day;
  if (elapsed < minute) return '刚刚';
  if (elapsed < hour) return `${Math.floor(elapsed / minute)} 分前`;
  if (elapsed < day) return `${Math.floor(elapsed / hour)} 时前`;
  if (elapsed < week) return `${Math.floor(elapsed / day)} 天前`;
  if (elapsed < month) return `${Math.floor(elapsed / week)} 周前`;
  if (elapsed < year) return `${Math.floor(elapsed / month)} 月前`;
  return `${Math.floor(elapsed / year)} 年前`;
}

export const resourceCategoryOptions: Record<string, string[]> = {
  全部: [],
  Artifact: ['Generic', 'Docker', 'Helm'],
  Repository: ['Git', 'Bundle'],
  Host: ['Direct', 'Agent'],
  Docker: ['Direct', 'Agent'],
  Kubernetes: ['Direct', 'Agent'],
  Nacos: ['Direct', 'Agent'],
  Nginx: ['Direct', 'Agent'],
  TongHttpServer: ['Direct', 'Agent'],
  PostgreSQL: ['Direct', 'Agent'],
  Oracle: ['Direct', 'Agent'],
  MySQL: ['Direct', 'Agent'],
  OceanBase: ['Direct', 'Agent'],
  Redis: ['Direct', 'Agent'],
  TongRDS: ['Direct', 'Agent'],
  Kafka: ['Direct', 'Agent'],
  RabbitMQ: ['Direct', 'Agent'],
  ElasticSearch: ['Direct', 'Agent'],
  MinIO: ['Direct', 'Agent'],
  MCPServer: ['StreamHTTP', 'SSE'],
  Prometheus: ['Direct', 'Agent'],
  Loki: ['Direct', 'Agent'],
  Tempo: ['Direct', 'Agent'],
  Jaeger: ['Direct', 'Agent'],
  Skywalking: ['Direct', 'Agent'],
  Alertmanager: ['Direct', 'Agent']
};

export const resourceCatalogDomains: Record<string, string> = {
  Artifact: '工程库',
  Repository: '工程库',
  Host: '运行时',
  Docker: '运行时',
  Kubernetes: '运行时',
  Nacos: '微服务',
  Nginx: '微服务',
  TongHttpServer: '微服务',
  PostgreSQL: '数据库',
  MySQL: '数据库',
  Oracle: '数据库',
  OceanBase: '数据库',
  Redis: '中间件',
  Kafka: '中间件',
  RabbitMQ: '中间件',
  TongRDS: '中间件',
  ElasticSearch: '存储库',
  MinIO: '存储库',
  MCPServer: '大模型',
  Prometheus: '可观测',
  Loki: '可观测',
  Tempo: '可观测',
  Jaeger: '可观测',
  Skywalking: '可观测',
  Alertmanager: '可观测'
};

export const resourceCatalogDomainOrder = [
  '运行时',
  '大模型',
  '工程库',
  '数据库',
  '存储库',
  '中间件',
  '微服务',
  '可观测'
];

export const resourceCatalogTags: Record<string, string[]> = {
  Artifact: ['制品仓库'],
  Repository: ['代码仓库'],
  Host: ['运行时'],
  Docker: ['运行时'],
  Kubernetes: ['运行时'],
  Nacos: ['注册配置'],
  Nginx: ['前端代理'],
  TongHttpServer: ['前端代理'],
  PostgreSQL: ['数据库'],
  Oracle: ['数据库'],
  MySQL: ['数据库'],
  OceanBase: ['数据库'],
  TongRDS: ['缓存库'],
  Redis: ['缓存库'],
  Kafka: ['消息队列'],
  RabbitMQ: ['消息队列'],
  ElasticSearch: ['文档检索'],
  Elasticsearch: ['文档检索'],
  MinIO: ['对象存储'],
  MCPServer: ['模型工具'],
  Prometheus: ['指标度量'],
  Loki: ['日志检索'],
  Tempo: ['链路追踪'],
  Jaeger: ['链路追踪'],
  Skywalking: ['链路追踪'],
  Alertmanager: ['事件告警']
};

export function resourceCatalogTagsFor(resource: ResourceShape | string) {
  const kind = typeof resource === 'string' ? resource : resource.kind;
  const category = resourceCategoryFor(
    typeof resource === 'string' ? { kind: resource } : resource
  );
  return resourceCatalogTags[kind] ?? resourceCatalogTags[category] ?? [];
}

export const resourceCatalogTagOptions = [
  ...new Set(Object.values(resourceCatalogTags).flat())
];

export function resourceCatalogTagClass(tag: string) {
  const classes: Record<string, string> = {
    制品仓库: 'artifact',
    代码仓库: 'code',
    运行时: 'runtime',
    注册配置: 'registry',
    前端代理: 'gateway',
    数据库: 'database',
    缓存库: 'cache',
    消息队列: 'message',
    文档检索: 'search',
    对象存储: 'storage',
    模型工具: 'ai',
    指标度量: 'metric',
    日志检索: 'log',
    链路追踪: 'trace',
    事件告警: 'alarm'
  };
  return classes[tag] ?? 'default';
}

type ResourceShape = {
  kind: string;
  subtype?: string;
  config?: Record<string, unknown>;
};

export function resourceDomainFor(resource: ResourceShape | string) {
  const kind = typeof resource === 'string' ? resource : resource.kind;
  const category = resourceCategoryFor(
    typeof resource === 'string' ? { kind: resource } : resource
  );
  return resourceCatalogDomains[kind] ?? resourceCatalogDomains[category] ?? '其他类';
}

export function connectorCapabilityName(capability: ConnectorCapability) {
  const names: Record<ConnectorCapability, string> = {
    kubernetes_read: '读取 Kubernetes',
    host_info: '读取主机信息',
    host_metrics: '读取主机指标',
    host_processes: '读取主机进程',
    host_file_logs: '读取主机文件日志',
    host_health: '检查主机健康',
    query_metrics: '查询指标',
    query_logs: '查询日志',
    query_traces: '查询链路',
    get_alerts: '读取告警'
  };
  return names[capability] ?? capability;
}

export function resourceCategoryFor(resource: ResourceShape) {
  return resource.kind === 'Elasticsearch' ? 'ElasticSearch' : resource.kind;
}

export function resourceSubtypeFor(resource: ResourceShape) {
  const fallback: Record<string, string> = {
    Artifact: 'Generic',
    Kubernetes: 'Direct',
    Nacos: 'Direct',
    Nginx: 'Direct',
    TongHttpServer: 'Direct',
    ElasticSearch: 'Direct',
    Host: 'Direct',
    Docker: 'Direct',
    Redis: 'Direct',
    Kafka: 'Direct',
    Elasticsearch: 'Direct',
    RabbitMQ: 'Direct',
    MinIO: 'Direct',
    TongRDS: 'Direct',
    OceanBase: 'Direct',
    Oracle: 'Direct',
    MySQL: 'Direct',
    PostgreSQL: 'Direct',
    Repository: 'Git',
    MCPServer: 'StreamHTTP',
    Prometheus: 'Direct',
    Loki: 'Direct',
    Tempo: 'Direct',
    Jaeger: 'Direct',
    Skywalking: 'Direct',
    Alertmanager: 'Direct'
  };
  const explicit = String(resource.subtype || resource.config?.subtype || '');
  if (
    [
      'Host',
      'Docker',
      'Kubernetes',
      'Nacos',
      'Nginx',
      'TongHttpServer',
      'Redis',
      'TongRDS',
      'Kafka',
      'RabbitMQ',
      'Elasticsearch',
      'ElasticSearch',
      'OceanBase',
      'Oracle',
      'MySQL',
      'PostgreSQL',
      'MinIO',
      'Prometheus',
      'Loki',
      'Tempo',
      'Jaeger',
      'Skywalking',
      'Alertmanager'
    ].includes(resource.kind)
  )
    return explicit || 'Direct';
  return String(
    explicit ||
      resource.config?.provider ||
      fallback[resource.kind] ||
      resource.kind
  );
}

export function resourceSubtypeOptionsFor(resource: Resource) {
  const current = resourceSubtypeFor(resource);
  const options = resourceCategoryOptions[resourceCategoryFor(resource)] ?? [];
  return options.includes(current) ? options : [current, ...options];
}

export function resourceCategoryIcon(category: string) {
  const icons: Record<string, string> = {
    全部: 'lucide:Circle',
    Artifact: 'lucide:Package',
    Repository: 'lucide:FolderGit2',
    Host: 'lucide:Server',
    Docker: 'iconify:simple-icons:docker',
    Kubernetes: 'iconify:simple-icons:kubernetes',
    Redis: 'iconify:simple-icons:redis',
    TongRDS: 'lucide:Database',
    Kafka: 'iconify:simple-icons:apache-kafka',
    RabbitMQ: 'iconify:simple-icons:rabbitmq',
    Elasticsearch: 'iconify:simple-icons:elasticsearch',
    OceanBase: 'lucide:Database',
    Oracle: 'lucide:Database',
    MySQL: 'lucide:Database',
    PostgreSQL: 'iconify:simple-icons:postgresql',
    MinIO: 'iconify:simple-icons:minio',
    MCPServer: 'lucide:Waypoints',
    Prometheus: 'lucide:ChartNoAxesCombined',
    Loki: 'lucide:FileSearch',
    Tempo: 'lucide:Route',
    Jaeger: 'lucide:Route',
    Skywalking: 'lucide:Route',
    Alertmanager: 'lucide:Bell'
  };
  return icons[category] ?? 'lucide:Circle';
}

export function resourceSchemaForSelection(
  schemas: ResourceSchema[],
  category: string,
  subtype: string
) {
  return (
    schemas.find(
      (schema) =>
        resourceCategoryFor(schema) === category &&
        resourceSubtypeFor(schema) === subtype
    ) ??
    schemas.find((schema) => resourceCategoryFor(schema) === category) ??
    schemas[0] ??
    null
  );
}

export function resourceEndpointFor(resource: ResourceShape) {
  if (resource.kind === 'Repository') {
    return (
      String(resource.config?.endpoint ?? '').trim() ||
      String(resource.config?.url ?? '').trim() ||
      '未设置仓库地址'
    );
  }
  if (resource.kind === 'PostgreSQL') {
    if (String(resource.subtype ?? '').toLowerCase() === 'agent')
      return 'MCPServer 代理';
    const host = String(resource.config?.host ?? '').trim();
    const port = Number(resource.config?.port ?? 5432);
    const database = String(resource.config?.database ?? '').trim();
    return host ? `${host}:${port}/${database}` : '未设置 PostgreSQL 地址';
  }
  if (resource.kind === 'MySQL') {
    if (String(resource.subtype ?? '').toLowerCase() === 'agent')
      return 'MCPServer 代理';
    const host = String(resource.config?.host ?? '').trim();
    const port = Number(resource.config?.port ?? 3306);
    const database = String(resource.config?.database ?? '').trim();
    return host ? `${host}:${port}/${database}` : '未设置 MySQL 地址';
  }
  if (resource.kind === 'Oracle') {
    if (String(resource.subtype ?? '').toLowerCase() === 'agent')
      return 'MCPServer 代理';
    const host = String(resource.config?.host ?? '').trim();
    const port = Number(resource.config?.port ?? 1521);
    const service = String(
      resource.config?.service_name ?? resource.config?.sid ?? ''
    ).trim();
    return host
      ? `${host}:${port}/${service || '未设置服务'}`
      : '未设置 Oracle 地址';
  }
  if (resource.kind === 'RabbitMQ') {
    if (String(resource.subtype ?? '').toLowerCase() === 'agent')
      return 'MCPServer 代理';
    return (
      String(resource.config?.url ?? '').trim() ||
      '未设置 RabbitMQ Management API'
    );
  }
  if (resource.kind === 'MinIO') {
    if (String(resource.subtype ?? '').toLowerCase() === 'agent')
      return 'MCPServer 代理';
    const endpoint = String(resource.config?.endpoint ?? '').trim();
    return endpoint || '未设置 MinIO Endpoint';
  }
  if (resource.kind === 'Redis') {
    if (String(resource.subtype ?? '').toLowerCase() === 'agent')
      return 'MCPServer 代理';
    const host = String(resource.config?.host ?? '').trim();
    const port = Number(resource.config?.port ?? 6379);
    const database = Number(resource.config?.database ?? 0);
    return host ? `${host}:${port}/db${database}` : '未设置 Redis 地址';
  }
  if (resource.kind === 'Nacos') {
    if (String(resource.subtype ?? '').toLowerCase() === 'agent')
      return 'MCPServer 代理';
    const host = String(resource.config?.host ?? '').trim();
    const scheme = String(resource.config?.scheme ?? 'http');
    const port = Number(resource.config?.port ?? 8848);
    const path = String(resource.config?.context_path ?? '/nacos').replace(
      /\/$/,
      ''
    );
    return host ? `${scheme}://${host}:${port}${path}` : '未设置 Nacos 地址';
  }
  if (resource.kind === 'Kafka') {
    if (String(resource.subtype ?? '').toLowerCase() === 'agent')
      return 'MCPServer 代理';
    const brokers = Array.isArray(resource.config?.brokers)
      ? resource.config.brokers.map(String).filter(Boolean)
      : [];
    return brokers.length ? brokers.join(', ') : '未设置 Kafka Broker';
  }
  if (resource.kind === 'Elasticsearch' || resource.kind === 'ElasticSearch') {
    if (String(resource.subtype ?? '').toLowerCase() === 'agent')
      return 'MCPServer 代理';
    return (
      String(resource.config?.url ?? '').trim() || '未设置 Elasticsearch URL'
    );
  }
  if (resource.kind === 'Host') {
    if (String(resource.subtype ?? '').toLowerCase() === 'agent')
      return 'MCPServer 代理';
    const host = String(resource.config?.host ?? '').trim();
    if (!host) return '本机 Linux';
    const normalizedHost =
      host.includes(':') && !host.startsWith('[') ? `[${host}]` : host;
    const port = Number(resource.config?.port ?? 22);
    const suffix =
      Number.isFinite(port) && port > 0 && port !== 22 ? `:${port}` : '';
    return `ssh://${normalizedHost}${suffix}`;
  }
  if (resource.kind === 'Docker') {
    const mode = String(resource.subtype ?? '').toLowerCase();
    if (mode === 'agent') return 'MCPServer 代理';
    return String(resource.config?.host ?? '本机默认 Unix Socket');
  }
  if (resource.kind === 'Kubernetes') {
    if (String(resource.subtype ?? '').toLowerCase() === 'agent')
      return 'MCPServer 代理';
    const mode = String(
      resource.config?.connection_mode ??
        (resource.config?.server ? 'endpoint' : 'kubeconfig')
    ).toLowerCase();
    if (mode === 'kubeconfig') return '由 kubeconfig 提供 API Server';
    return (
      String(resource.config?.server ?? '').trim() ||
      '未设置 Kubernetes API Server'
    );
  }
  return String(
    resource.config?.url ??
      resource.config?.endpoint ??
      resource.config?.host ??
      '未设置端点'
  );
}
