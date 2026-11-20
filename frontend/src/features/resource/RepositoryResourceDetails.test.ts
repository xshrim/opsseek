import { afterEach, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';
import RepositoryResourceDetails from './RepositoryResourceDetails.svelte';
import type { Resource } from '../../lib/api';

let component: ReturnType<typeof mount>;
afterEach(async () => {
  if (component) await unmount(component);
  document.body.innerHTML = '';
  vi.unstubAllGlobals();
});

it('shows the concrete backend reason when an existing repository preview fails', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => ({
      ok: false,
      json: async () => ({
        error: { message: 'git clone: authentication failed' }
      })
    }))
  );
  component = mount(RepositoryResourceDetails, {
    target: document.body,
    props: {
      resource: { id: 'repo', kind: 'Repository', subtype: 'Git' } as Resource
    }
  });
  flushSync();
  await vi.waitFor(() =>
    expect(document.querySelector('.preview-empty')?.textContent).toContain(
      'git clone: authentication failed'
    )
  );
});

it('previews a revision with GET and never persists it, including for read-only users', async () => {
  const first = 'a'.repeat(40),
    second = 'b'.repeat(40);
  const fetchMock = vi.fn(async () => ({
    ok: true,
    json: async () => ({
      default_branch: 'main',
      commit_id: first,
      branches: [
        { branch: 'main', is_default: true, commits: [first, second] }
      ],
      entries: [{ path: 'README.md', type: 'file', size: 12 }]
    })
  }));
  vi.stubGlobal('fetch', fetchMock);
  const resource = {
    id: 'repo',
    kind: 'Repository',
    scope_id: 'scope',
    schema_version: 1,
    name: 'Repository',
    labels: {},
    status: 'active',
    subtype: 'Git',
    config: { endpoint: `main:${first}` },
    created_at: '',
    updated_at: ''
  } as Resource;
  component = mount(RepositoryResourceDetails, {
    target: document.body,
    props: { resource, canUpdate: true }
  });
  flushSync();
  await vi.waitFor(() =>
    expect(document.querySelector('.grouped-select-trigger')).not.toBeNull()
  );
  await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
  document.querySelector<HTMLButtonElement>('.grouped-select-trigger')!.click();
  flushSync();
  await vi.waitFor(() =>
    expect(document.querySelector('.group-list button')?.textContent).toContain(
      'main'
    )
  );
  document
    .querySelector<HTMLButtonElement>(`.option-list button[data-tooltip="${second}"]`)!
    .click();
  await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
  const requests = fetchMock.mock.calls as unknown as [string, RequestInit][];
  expect(String(requests[1][0])).toContain(`branch=main&commit_id=${second}`);
  expect(
    requests.every(([, options]) => !options.method || options.method === 'GET')
  ).toBe(true);
  expect(resource.config?.endpoint).toBe(`main:${first}`);
});

it('shows saved bundle fields and preview without an upload control', async () => {
  const first = 'a'.repeat(40);
  const resource = {
    id: 'repo',
    kind: 'Repository',
    subtype: 'Bundle',
    scope_id: 'scope',
    schema_version: 1,
    name: 'Repository',
    labels: {},
    status: 'active',
    created_at: '',
    updated_at: '',
    config: {
      default_branch: 'main',
      endpoint: `repo [main·${first}]`,
      storage_backend: 'postgres',
      storage_key: 'repo'
    }
  } as Resource;
  const fetchMock = vi.fn(async () => ({
    ok: true,
    json: async () => ({
      default_branch: 'main',
      commit_id: first,
      branches: [{ branch: 'main', commits: [first] }],
      entries: []
    })
  }));
  vi.stubGlobal('fetch', fetchMock);
  component = mount(RepositoryResourceDetails, {
    target: document.body,
    props: { resource, canUpdate: true }
  });
  flushSync();
  await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
  expect(document.querySelector('input[type="file"]')).toBeNull();
  expect(document.querySelector('.upload-button')).toBeNull();
  const details = document.querySelector('.provider-resource-meta')!;
  expect(details.textContent).toContain('postgres');
  expect(details.textContent).toContain(`repo [main·${first}]`);
  expect(details.textContent).toContain('已上传');
  expect(resource.config?.endpoint).toBe(`repo [main·${first}]`);
  expect(resource.config?.default_branch).toBe('main');
});
