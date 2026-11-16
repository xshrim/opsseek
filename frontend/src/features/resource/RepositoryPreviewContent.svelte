<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import {
    ChevronDown,
    ChevronRight,
    File as FileIcon,
    Folder,
    List,
    ListTree,
    RefreshCw
  } from 'lucide-svelte';
  import GroupedSelect from '../../components/GroupedSelect.svelte';
  import { formatFileSize } from '../../lib/format';
  import {
    buildRepositoryPreviewTree,
    repositoryRevisionGroups,
    visibleRepositoryPreviewNodes,
    type RepositoryBranchRevision,
    type RepositoryPreviewEntry
  } from './repositoryPreview';

  export let title = '仓库预览';
  export let subtitle = '';
  export let entries: RepositoryPreviewEntry[] = [];
  export let branches: RepositoryBranchRevision[] = [];
  export let selectedBranch = '';
  export let selectedCommit = '';
  export let revisionEditable = false;
  export let showRevisionSelector = true;
  export let showRefresh = false;
  export let refreshLabel = '刷新';
  export let showPreviewArea = true;
  export let busy = false;
  export let failed = false;
  export let failureReason = '';

  const dispatch = createEventDispatcher<{
    refresh: void;
    revisionchange: { branch: string; commitID: string };
  }>();
  let mode: 'list' | 'tree' = 'tree';
  let expandedPaths = new Set<string>();
  $: selectedBranchOption = branches.find(
    (option) => option.branch === selectedBranch
  );
  $: if (branches.length && !selectedBranch) {
    selectedBranch =
      branches.find((option) => option.is_default)?.branch ??
      branches[0].branch;
  }
  $: if (selectedBranchOption && !selectedCommit)
    selectedCommit = selectedBranchOption.commits[0] ?? '';
  $: revisionGroups = repositoryRevisionGroups(
    branches,
    selectedBranch,
    selectedCommit
  );
  $: tree = buildRepositoryPreviewTree(entries);
  $: flattenedTree = visibleRepositoryPreviewNodes(tree, expandedPaths);

  function toggle(path: string) {
    const next = new Set(expandedPaths);
    if (next.has(path)) next.delete(path);
    else next.add(path);
    expandedPaths = next;
  }

  function changeRevision(
    event: CustomEvent<{ group: string; value: string }>
  ) {
    selectedBranch = event.detail.group;
    selectedCommit = event.detail.value;
    dispatch('revisionchange', {
      branch: selectedBranch,
      commitID: selectedCommit
    });
  }
</script>

<section class="repository-preview">
  <header class="preview-heading">
    <div class="preview-title">
      <strong>{title}</strong>
      {#if subtitle}<small>{subtitle}</small>{/if}
    </div>
    <div class="preview-actions">
      {#if showPreviewArea && showRevisionSelector && branches.length}
        <GroupedSelect
          groups={revisionGroups}
          group={selectedBranch}
          value={selectedCommit}
          groupLabel="分支"
          optionLabel="Commit ID"
          ariaLabel="选择分支和 Commit ID"
          disabled={!revisionEditable || busy}
          on:change={changeRevision}
        />
      {/if}
      {#if showRefresh}
        <button
          class="secondary refresh-button"
          type="button"
          disabled={busy}
          on:click={() => dispatch('refresh')}
        >
          <RefreshCw size={14} aria-hidden="true" />
          {busy
            ? refreshLabel === '预览'
              ? '预览中…'
              : '刷新中…'
            : refreshLabel}
        </button>
      {/if}
    </div>
  </header>

  {#if showPreviewArea}
    <div class="preview-panel">
      <div
        class="preview-mode segmented-control"
        role="group"
        aria-label="仓库预览模式"
      >
        <button
          class:active={mode === 'list'}
          type="button"
          aria-pressed={mode === 'list'}
          on:click={() => (mode = 'list')}
          ><List size={14} aria-hidden="true" />列表</button
        >
        <button
          class:active={mode === 'tree'}
          type="button"
          aria-pressed={mode === 'tree'}
          on:click={() => (mode = 'tree')}
          ><ListTree size={14} aria-hidden="true" />目录树</button
        >
      </div>
      {#if entries.length}
        {#if mode === 'list'}
          <ul class="repository-file-list" aria-label="仓库文件列表">
            {#each entries as entry (entry.path)}
              <li>
                <code>{entry.path}</code>{#if entry.size !== undefined}<small
                    >{formatFileSize(entry.size)}</small
                  >{/if}
              </li>
            {/each}
          </ul>
        {:else}
          <ul class="repository-file-tree" aria-label="仓库目录树">
            {#each flattenedTree as node (node.path)}
              <li style:--depth={node.depth}>
                {#if node.children.length}
                  <button
                    class="tree-toggle"
                    type="button"
                    aria-expanded={expandedPaths.has(node.path)}
                    aria-label={(expandedPaths.has(node.path)
                      ? '折叠'
                      : '展开') +
                      ' ' +
                      node.name}
                    on:click={() => toggle(node.path)}
                  >
                    {#if expandedPaths.has(node.path)}<ChevronDown
                        size={14}
                        aria-hidden="true"
                      />{:else}<ChevronRight
                        size={14}
                        aria-hidden="true"
                      />{/if}
                    <Folder size={14} aria-hidden="true" /><span
                      >{node.name}</span
                    >
                  </button>
                {:else}
                  <span class="tree-file"
                    ><FileIcon size={14} aria-hidden="true" /><code
                      >{node.name}</code
                    ></span
                  >
                {/if}
                {#if node.size !== undefined}<small
                    >{formatFileSize(node.size)}</small
                  >{/if}
              </li>
            {/each}
          </ul>
        {/if}
      {:else if busy}
        <p class="preview-empty muted">正在读取仓库内容…</p>
      {:else if failed}
        <p class="preview-empty muted">
          {failureReason || '读取失败，请根据标题栏提示处理后重试。'}
        </p>
      {:else}
        <p class="preview-empty muted">
          仓库没有可显示的文件，或所选分支为空。
        </p>
      {/if}
    </div>
  {/if}
</section>

<style>
  .repository-preview {
    display: grid;
    gap: 9px;
    width: 100%;
    min-width: 0;
  }
  .preview-heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    min-width: 0;
  }
  .preview-title {
    display: grid;
    flex: 0 1 auto;
    gap: 3px;
    min-width: 0;
  }
  .preview-title strong {
    color: var(--theme-fg-strong);
    font-size: 13px;
  }
  .preview-title small {
    color: var(--theme-fg-muted);
    font-size: 11px;
  }
  .preview-actions {
    display: flex;
    flex: 1 1 auto;
    align-items: center;
    justify-content: flex-end;
    gap: 7px;
    min-width: 0;
  }
  .refresh-button {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    min-height: 34px;
    flex: 0 0 auto;
    white-space: nowrap;
  }
  .preview-panel {
    position: relative;
    min-width: 0;
  }
  .preview-mode {
    position: absolute;
    z-index: 1;
    top: 7px;
    right: 8px;
    margin: 0;
    background: var(--theme-bg-raised);
    opacity: 0;
    visibility: hidden;
    transition: opacity 120ms ease;
  }
  .preview-panel:hover .preview-mode,
  .preview-panel:focus-within .preview-mode {
    opacity: 1;
    visibility: visible;
  }
  .preview-mode button {
    min-height: 28px;
    gap: 5px;
    padding: 0 8px;
  }
  .repository-file-list,
  .repository-file-tree {
    display: grid;
    gap: 4px;
    max-height: 360px;
    overflow: auto;
    margin: 0;
    padding: 10px 12px;
    border: 1px solid var(--theme-border);
    border-radius: 4px;
    background: var(--theme-bg-sunken);
    list-style: none;
  }
  .preview-empty {
    min-height: 52px;
    margin: 0;
    padding: 12px;
    border: 1px solid var(--theme-border);
    border-radius: 4px;
    background: var(--theme-bg-sunken);
  }
  .repository-file-list li,
  .repository-file-tree li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    min-height: 22px;
    color: var(--theme-fg-strong);
    font-size: 12px;
  }
  .repository-file-list small,
  .repository-file-tree small {
    color: var(--theme-fg-muted);
    font-size: 11px;
  }
  .repository-file-list code,
  .tree-file code {
    overflow-wrap: anywhere;
  }
  .repository-file-tree li {
    padding-left: calc(var(--depth) * 18px);
  }
  .tree-toggle,
  .tree-file {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    min-width: 0;
    color: var(--theme-fg-strong);
  }
  .tree-toggle {
    padding: 0;
    background: none;
    border: 0;
    text-align: left;
  }
  .tree-toggle span {
    overflow-wrap: anywhere;
  }
  .tree-file {
    padding-left: 19px;
  }
  .repository-file-tree :global(svg) {
    flex: 0 0 auto;
    color: var(--theme-fg-muted);
  }
</style>
