import { describe, expect, it } from 'vitest';
import { formatFileSize } from '../../lib/format';
import {
  buildRepositoryPreviewTree,
  repositoryRevisionGroups,
  repositoryRevisionFromEndpoint,
  visibleRepositoryPreviewNodes
} from './repositoryPreview';
import type { RepositoryBranchRevision } from './repositoryPreview';

describe('repository preview tree', () => {
  it('sorts branches and commits by commit time without changing inputs or losing an older pinned commit', () => {
    const branches: RepositoryBranchRevision[] = [
      {
        branch: 'main',
        is_default: true,
        commits: ['old', 'new'],
        commit_times: { old: 1, new: 3 }
      },
      {
        branch: 'release',
        is_default: false,
        commits: ['recent'],
        commit_times: { recent: 4 }
      }
    ];
    const groups = repositoryRevisionGroups(branches, 'main', 'pinned');
    expect(groups.map((group) => group.value)).toEqual(['release', 'main']);
    expect(groups[1].options.map((option) => option.value)).toEqual([
      'new',
      'old',
      'pinned'
    ]);
    expect(branches[0].commits).toEqual(['old', 'new']);
  });
  it('builds nested directories and exposes children only when expanded', () => {
    const tree = buildRepositoryPreviewTree([
      { path: 'src/main.ts', type: 'file', size: 10 },
      { path: 'src/lib/util.ts', type: 'file', size: 20 },
      { path: 'README.md', type: 'file', size: 5 }
    ]);

    expect(tree.map(({ path }) => path)).toEqual(['src', 'README.md']);
    expect(tree[0].children.map(({ path }) => path)).toEqual([
      'src/main.ts',
      'src/lib'
    ]);
    expect(
      visibleRepositoryPreviewNodes(tree, new Set()).map(({ path }) => path)
    ).toEqual(['src', 'README.md']);
    expect(
      visibleRepositoryPreviewNodes(tree, new Set(['src'])).map(
        ({ path }) => path
      )
    ).toEqual(['src', 'src/main.ts', 'src/lib', 'README.md']);
  });

  it('formats file sizes with readable units', () => {
    expect(formatFileSize(0)).toBe('0 B');
    expect(formatFileSize(512)).toBe('512 B');
    expect(formatFileSize(1024)).toBe('1 KB');
    expect(formatFileSize(1536)).toBe('1.5 KB');
    expect(formatFileSize(1048576)).toBe('1 MB');
  });

  it('reads saved branch and commit selections for Git and Bundle endpoints', () => {
    const commit = 'a'.repeat(40);
    expect(
      repositoryRevisionFromEndpoint(
        'Bundle',
        '',
        `9f21d5c7-5cf3-41d5-a0ba-55063fd40722 [main·${commit}]`
      )
    ).toEqual({ branch: 'main', commitID: commit });
    expect(
      repositoryRevisionFromEndpoint(
        'Git',
        'https://git.example/repo.git',
        `https://git.example/repo.git [release·${commit}]`
      )
    ).toEqual({ branch: 'release', commitID: commit });
    expect(
      repositoryRevisionFromEndpoint(
        'Git',
        'https://git.example/repo.git',
        `https://git.example/repo.git (release:${commit})`
      )
    ).toEqual({ branch: 'release', commitID: commit });
  });
});
