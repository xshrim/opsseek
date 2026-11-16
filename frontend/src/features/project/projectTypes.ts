import type { Resource } from '../../lib/api';

export type ApplicationRuntimeKind =
  'virtual_machine' | 'containerized' | 'cloud_native';

export type RuntimeResourceKind = 'Host' | 'Docker' | 'Kubernetes';

export type ApplicationResourceCreationRequest = {
  id: string;
  kind: string;
  scopeId: string;
  instanceId?: string;
  coverageTarget?: string;
};

export type ApplicationResourceCreationResult = {
  requestId: string;
  instanceId?: string;
  resource: Resource;
};

export type ApplicationInstanceDraft = {
  id: string;
  persistedId?: string;
  name: string;
  targetResourceId: string;
  selector: Record<string, unknown>;
  logBinding?: Record<string, unknown>;
  status?: string;
};

export type ApplicationDraft = {
  name: string;
  code: string;
  description: string;
  icon: string;
  labels: Record<string, string>;
  runtimeKind: ApplicationRuntimeKind;
  externalUid?: string;
  critical: boolean;
  technicalOwnerIds: string[];
  operationsOwnerIds: string[];
  dependencies?: Array<{
    dependency_kind: string;
    target_resource_id: string;
    binding?: Record<string, string>;
    required?: boolean;
  }>;
  relations?: Array<{
    target_application_id: string;
    relation_kind: 'depends_on';
    status?: string;
  }>;
  externalDependencies?: Array<{
    projectName: string;
    description: string;
  }>;
  instances: ApplicationInstanceDraft[];
};

export const runtimeLabel = (kind: ApplicationRuntimeKind) =>
  kind === 'virtual_machine'
    ? 'Host'
    : kind === 'containerized'
      ? 'Docker'
      : 'Kubernetes';

export const runtimeResourceKind = (kind: ApplicationRuntimeKind) =>
  runtimeLabel(kind);

export const matchesSearchFields = (
  searchable: Record<string, string>,
  freeQuery: string,
  fields: Record<string, string>
) =>
  Object.entries(fields).every(
    ([key, value]) =>
      !value ||
      String(searchable[key] ?? '')
        .toLocaleLowerCase()
        .includes(value.toLocaleLowerCase())
  ) &&
  (!freeQuery ||
    Object.values(searchable)
      .join(' ')
      .toLocaleLowerCase()
      .includes(freeQuery.toLocaleLowerCase()));

export const applicationKeywordFromSelector = (
  kind: ApplicationRuntimeKind,
  selector: Record<string, unknown> | undefined
) => {
  if (kind === 'virtual_machine')
    return String(selector?.process_command_line ?? '')
      .trim()
      .split(/\s+/)[0]
      ?.slice(0, 128) ?? '';
  if (kind === 'containerized')
    return String(selector?.container_name ?? '');
  return '';
};

export const preserveConfiguredValue = (current: string, fallback: string) =>
  current.trim() ? current : fallback;

export const normalizeRepositoryBinding = (
  binding: Record<string, string>
) => {
  const result = { ...binding };
  const legacyRevision = result.revision?.trim() ?? '';
  delete result.revision;
  if (legacyRevision && !result.branch && !result.commit_id) {
    if (/^[\da-f]{4,64}$/i.test(legacyRevision))
      result.commit_id = legacyRevision;
    else result.branch = legacyRevision;
  }
  return result;
};

export const slugify = (value: string) => {
  const slug = value
    .trim()
    .toLocaleLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
  return slug || `application-${Date.now()}`;
};

export const resourcesForRuntime = (
  resources: Resource[],
  kind: ApplicationRuntimeKind
) =>
  resources.filter((resource) => resource.kind === runtimeResourceKind(kind));
