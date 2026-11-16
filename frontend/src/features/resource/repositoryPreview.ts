export type RepositoryPreviewEntry = {
  path: string;
  type: string;
  size?: number;
};

export type RepositoryBranchRevision = {
  branch: string;
  commits: string[];
  commit_times?: Record<string, number>;
  is_default: boolean;
};

export function repositoryRevisionGroups(
  branches: RepositoryBranchRevision[],
  selectedBranch: string,
  selectedCommit: string
) {
  return branches
    .map((item) => {
      const commits =
        item.branch === selectedBranch &&
        selectedCommit &&
        !item.commits.includes(selectedCommit)
          ? [...item.commits, selectedCommit]
          : [...item.commits];
      const timestamp = (commit: string) => item.commit_times?.[commit] ?? 0;
      commits.sort((a, b) => timestamp(b) - timestamp(a));
      return {
        value: item.branch,
        label: item.branch,
        latest: Math.max(0, ...commits.map(timestamp)),
        options: commits.map((commit) => ({
          value: commit,
          label: commit.slice(0, 12),
          description: timestamp(commit)
            ? new Date(timestamp(commit) * 1000).toLocaleString()
            : ''
        }))
      };
    })
    .sort((a, b) => b.latest - a.latest);
}

export function repositoryRevisionFromEndpoint(
  subtype: string,
  url: string,
  endpoint: string
) {
  let selection = endpoint.trim();
  if (subtype.toLowerCase() === 'git') {
    const prefix = `${url.trim()} [`;
    if (selection.startsWith(prefix) && selection.endsWith(']')) {
      selection = selection.slice(prefix.length, -1);
      const separator = selection.lastIndexOf('·');
      if (separator < 1) return { branch: '', commitID: '' };
      const branch = selection.slice(0, separator);
      const commitID = selection.slice(separator + 1);
      return /^(?:[a-f0-9]{40}|[a-f0-9]{64})$/i.test(commitID)
        ? { branch, commitID }
        : { branch: '', commitID: '' };
    }
    const legacyPrefix = `${url.trim()} (`;
    if (!selection.startsWith(legacyPrefix) || !selection.endsWith(')'))
      return { branch: '', commitID: '' };
    selection = selection.slice(legacyPrefix.length, -1);
  } else if (selection.includes(' [') && selection.endsWith(']')) {
    selection = selection.slice(selection.lastIndexOf(' [') + 2, -1);
    const separator = selection.lastIndexOf('·');
    if (separator < 1) return { branch: '', commitID: '' };
    const branch = selection.slice(0, separator);
    const commitID = selection.slice(separator + 1);
    return /^(?:[a-f0-9]{40}|[a-f0-9]{64})$/i.test(commitID)
      ? { branch, commitID }
      : { branch: '', commitID: '' };
  }
  const separator = selection.lastIndexOf(':');
  if (separator < 1) return { branch: '', commitID: '' };
  const branch = selection.slice(0, separator);
  const commitID = selection.slice(separator + 1);
  return /^(?:[a-f0-9]{40}|[a-f0-9]{64})$/i.test(commitID)
    ? { branch, commitID }
    : { branch: '', commitID: '' };
}

export type RepositoryPreviewNode = {
  name: string;
  path: string;
  type: string;
  size?: number;
  children: RepositoryPreviewNode[];
  depth: number;
};

export function buildRepositoryPreviewTree(entries: RepositoryPreviewEntry[]) {
  const roots: RepositoryPreviewNode[] = [];
  const childMaps = new Map<
    RepositoryPreviewNode[],
    Map<string, RepositoryPreviewNode>
  >([[roots, new Map()]]);

  for (const entry of entries) {
    const parts = entry.path.split('/').filter(Boolean);
    let children = roots;
    for (const [index, name] of parts.entries()) {
      let siblings = childMaps.get(children);
      if (!siblings) childMaps.set(children, (siblings = new Map()));
      let node = siblings.get(name);
      const path = parts.slice(0, index + 1).join('/');
      if (!node) {
        node = {
          name,
          path,
          type: index === parts.length - 1 ? entry.type : 'directory',
          size: index === parts.length - 1 ? entry.size : undefined,
          children: [],
          depth: index
        };
        siblings.set(name, node);
        children.push(node);
      }
      children = node.children;
    }
  }

  return roots;
}

export function visibleRepositoryPreviewNodes(
  nodes: RepositoryPreviewNode[],
  expandedPaths: Set<string>
): RepositoryPreviewNode[] {
  return nodes.flatMap((node) => [
    node,
    ...(expandedPaths.has(node.path)
      ? visibleRepositoryPreviewNodes(node.children, expandedPaths)
      : [])
  ]);
}
