import { describe, expect, it } from 'vitest';
import {
  resourceCatalogTagOptions,
  resourceCatalogTagClass,
  resourceCatalogTagsFor,
  resourceCategoryFor,
  resourceCategoryOptions,
  resourceCatalogDomains,
  resourceDomainFor,
  resourceSubtypeFor
} from './resourceCatalog';

describe('resource catalog types and tags', () => {
  it('uses the updated resource tags', () => {
    expect(resourceCatalogTagsFor('Artifact')).toEqual(['制品仓库']);
    expect(resourceCatalogTagsFor('Repository')).toEqual(['代码仓库']);
    expect(resourceCatalogTagsFor('Nacos')).toEqual(['注册配置']);
    expect(resourceCatalogTagsFor('Nginx')).toEqual(['前端代理']);
    expect(resourceCatalogTagsFor('TongHttpServer')).toEqual(['前端代理']);
    expect(resourceCatalogTagsFor('RabbitMQ')).toEqual(['消息队列']);
    expect(resourceCatalogTagsFor('Kafka')).toEqual(['消息队列']);
    expect(resourceCatalogTagsFor('Oracle')).toEqual(['数据库']);
    expect(resourceCatalogTagsFor('Redis')).toEqual(['缓存库']);
    expect(resourceCatalogTagsFor('TongRDS')).toEqual(['缓存库']);
    expect(resourceCatalogTagsFor('ElasticSearch')).toEqual(['文档检索']);
    expect(resourceCatalogTagsFor('MinIO')).toEqual(['对象存储']);
    expect(resourceCatalogTagsFor('MCPServer')).toEqual(['模型工具']);
    expect(resourceCatalogTagOptions).not.toContain('监控');
  });

  it('assigns a distinct color class to every catalog tag', () => {
    const classes = resourceCatalogTagOptions.map(resourceCatalogTagClass);
    expect(new Set(classes).size).toBe(resourceCatalogTagOptions.length);
    expect(classes).not.toContain('default');
  });

  it('offers concrete observability resource types with Direct and Agent modes', () => {
    expect(resourceCategoryOptions).not.toHaveProperty('Monitor');
    for (const kind of [
      'Prometheus',
      'Loki',
      'Tempo',
      'Jaeger',
      'Skywalking',
      'Alertmanager'
    ]) {
      expect(resourceCategoryOptions[kind]).toEqual(['Direct', 'Agent']);
      expect(resourceCategoryFor({ kind })).toBe(kind);
    }
    expect(resourceSubtypeFor({ kind: 'Tempo' })).toBe('Direct');
    expect(resourceCatalogTagsFor('Prometheus')).toEqual(['指标度量']);
    expect(resourceCatalogTagsFor('Loki')).toEqual(['日志检索']);
    expect(resourceCatalogTagsFor('Skywalking')).toEqual(['链路追踪']);
    expect(resourceCatalogTagsFor('Alertmanager')).toEqual(['事件告警']);
  });

  it('assigns every catalog resource type to one logical domain', () => {
    const kinds = Object.keys(resourceCategoryOptions).filter(
      (kind) => kind !== '全部'
    );
    expect(Object.keys(resourceCatalogDomains).sort()).toEqual(kinds.sort());
    expect(new Set(kinds.map((kind) => resourceDomainFor(kind))).size).toBe(8);
    expect(resourceDomainFor('Artifact')).toBe('工程库');
    expect(resourceDomainFor('Repository')).toBe('工程库');
    expect(resourceDomainFor('ElasticSearch')).toBe('存储库');
    expect(resourceDomainFor('MinIO')).toBe('存储库');
    expect(resourceDomainFor('Redis')).toBe('中间件');
  });
});
