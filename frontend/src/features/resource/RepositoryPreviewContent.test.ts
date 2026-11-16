import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';
import RepositoryPreviewContent from './RepositoryPreviewContent.svelte';

let component: ReturnType<typeof mount>;
afterEach(async () => {
  if (component) await unmount(component);
  document.body.innerHTML = '';
});

const branches = [
  { branch: 'main', is_default: true, commits: ['a'.repeat(40)] },
  { branch: 'release', is_default: false, commits: ['b'.repeat(40)] }
];

describe('repository preview component', () => {
  it('defaults to a read-only revision and directory tree with optional controls hidden', () => {
    component = mount(RepositoryPreviewContent, {
      target: document.body,
      props: {
        branches,
        entries: [{ path: 'README.md', type: 'file', size: 1536 }]
      }
    });
    flushSync();
    const selector = document.querySelector<HTMLButtonElement>(
      '.grouped-select-trigger'
    )!;
    expect(selector.textContent).toContain('main');
    expect(selector.textContent).toContain('a'.repeat(12));
    expect(selector.disabled).toBe(true);
    expect(
      document.querySelector('[aria-label="仓库目录树"]')?.textContent
    ).toContain('1.5 KB');
    expect(document.querySelector('input[type=file]')).toBeNull();
    expect(document.querySelector('.refresh-button')).toBeNull();
  });

  it('emits refresh while keeping uploads and empty previews out of the component', () => {
    const refresh = vi.fn();
    component = mount(RepositoryPreviewContent, {
      target: document.body,
      props: {
        branches,
        revisionEditable: true,
        showRefresh: true,
        showPreviewArea: false
      },
      events: { refresh }
    });
    flushSync();
    expect(document.querySelector('.preview-panel')).toBeNull();
    document.querySelector<HTMLButtonElement>('.refresh-button')!.click();
    expect(refresh).toHaveBeenCalledOnce();
    expect(document.querySelector('input[type=file]')).toBeNull();
    expect(document.body.textContent).not.toContain('该如何做');
  });

  it('selects the new branch tip and notifies its caller', () => {
    const revisionchange = vi.fn();
    component = mount(RepositoryPreviewContent, {
      target: document.body,
      props: { branches, revisionEditable: true },
      events: { revisionchange }
    });
    flushSync();
    document
      .querySelector<HTMLButtonElement>('.grouped-select-trigger')!
      .click();
    flushSync();
    const release = [
      ...document.querySelectorAll<HTMLButtonElement>('.group-list button')
    ].find((button) => button.textContent?.includes('release'))!;
    release.click();
    flushSync();
    expect(revisionchange).not.toHaveBeenCalled();
    document.querySelector<HTMLButtonElement>('.option-list button')!.click();
    flushSync();
    expect(revisionchange.mock.calls[0][0].detail).toEqual({
      branch: 'release',
      commitID: 'b'.repeat(40)
    });
    expect(
      document.querySelector('.grouped-select-trigger')?.textContent
    ).toContain('b'.repeat(12));
  });
});
