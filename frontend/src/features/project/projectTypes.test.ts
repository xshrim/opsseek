import { describe, expect, it } from 'vitest';
import {
  applicationKeywordFromSelector,
  javaArtifactFromCommandLine,
  matchesSearchFields,
  normalizeRepositoryBinding,
  preserveConfiguredValue,
  slugify
} from './projectTypes';

describe('application edit fields', () => {
  it('generates a stable identifier without adding uniqueness suffixes', () => {
    expect(slugify('Payments API')).toBe('payments-api');
    expect(slugify('付款服务')).toBe('application');
    expect(slugify('付款服务')).toBe(slugify('付款服务'));
  });

  it('restores the runtime-specific initial search keyword', () => {
    expect(
      applicationKeywordFromSelector('virtual_machine', {
        process_command_line: 'java -jar /srv/orders.jar'
      })
    ).toBe('java');
    expect(
      applicationKeywordFromSelector('containerized', {
        container_name: 'orders-api'
      })
    ).toBe('orders-api');
    expect(applicationKeywordFromSelector('cloud_native', {})).toBe('');
  });

  it('keeps configured values when switching edit candidates', () => {
    expect(preserveConfiguredValue('orders-api', 'billing-api')).toBe(
      'orders-api'
    );
    expect(preserveConfiguredValue('  ', 'billing-api')).toBe('billing-api');
  });

  it('extracts the Java jar filename without changing the process identity', () => {
    expect(javaArtifactFromCommandLine('java -jar /srv/apps/orders-api.jar --port 8080')).toBe('orders-api.jar');
    expect(javaArtifactFromCommandLine('java -jar "C:\\apps\\billing worker.jar"')).toBe('billing worker.jar');
    expect(javaArtifactFromCommandLine('java com.example.Main')).toBe('');
  });

  it('maps legacy repository revisions to branch or commit ID fields', () => {
    expect(
      normalizeRepositoryBinding({ directory: 'services/orders', revision: 'main' })
    ).toEqual({ directory: 'services/orders', branch: 'main' });
    expect(normalizeRepositoryBinding({ revision: 'a1b2c3d4' })).toEqual({
      commit_id: 'a1b2c3d4'
    });
    expect(
      normalizeRepositoryBinding({ revision: 'main', branch: 'release' })
    ).toEqual({ branch: 'release' });
  });
});

describe('application search', () => {
  const searchable = {
    name: 'Orders API',
    code: 'orders-api',
    runtime: 'Host virtual_machine',
    status: 'healthy',
    label: 'team=payments'
  };

  it('combines field filters and free text without matching missing fields', () => {
    expect(
      matchesSearchFields(searchable, 'payments', {
        name: 'orders',
        runtime: 'host'
      })
    ).toBe(true);
    expect(
      matchesSearchFields(searchable, '', { status: 'failed' })
    ).toBe(false);
    expect(
      matchesSearchFields(searchable, 'payments', { code: 'billing' })
    ).toBe(false);
  });
});
