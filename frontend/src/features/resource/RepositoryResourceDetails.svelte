<script lang="ts">
  import { onMount } from 'svelte';
  import type { Resource } from '../../lib/api';
  import { appURL } from '../../lib/health';
  import { repositoryRevisionFromEndpoint } from './repositoryPreview';
  import { resourceEndpointFor } from './resourceCatalog';
  import RepositoryPreviewContent from './RepositoryPreviewContent.svelte';
  import type {
    RepositoryBranchRevision,
    RepositoryPreviewEntry
  } from './repositoryPreview';

  export let resource: Resource;
  export let canUpdate = false;
  export let formatDate: (value: string) => string = (value) => value || '—';
  let busy = false;
  let failed = false;
  let failureReason = '';
  let previewEntries: RepositoryPreviewEntry[] = [];
  let branches: RepositoryBranchRevision[] = [];
  let selectedBranch = '';
  let selectedCommit = '';
  $: isBundle = String(resource.subtype ?? '').toLowerCase() === 'bundle';
  $: savedRevision = repositoryRevisionFromEndpoint(
    resource.subtype ?? '',
    String(resource.config?.url ?? ''),
    String(resource.config?.endpoint ?? '')
  );

  async function preview(branch = selectedBranch, commitID = selectedCommit) {
    if (busy) return;
    busy = true;
    failed = false;
    failureReason = '';
    try {
      const response = await fetch(
        appURL(
          'api/v1/resources/' +
            resource.id +
            '/repository/tree?' +
            new URLSearchParams({ branch, commit_id: commitID })
        ),
        { credentials: 'include' }
      );
      const data = await response.json().catch(() => ({}));
      if (!response.ok)
        throw new Error(
          data?.error?.message ?? data?.message ?? '仓库预览失败'
        );
      previewEntries = Array.isArray(data.entries) ? data.entries : [];
      branches = Array.isArray(data.branches) ? data.branches : [];
      selectedBranch = String(data.default_branch ?? '');
      selectedCommit = String(data.commit_id ?? '');
    } catch (error) {
      failed = true;
      failureReason =
        error instanceof Error ? error.message : '仓库预览失败，请稍后重试。';
      previewEntries = [];
    } finally {
      busy = false;
    }
  }

  async function selectRevision(
    event: CustomEvent<{ branch: string; commitID: string }>
  ) {
    const { branch, commitID } = event.detail;
    if (!branch || !commitID || busy) return;
    await preview(branch, commitID);
  }

  onMount(() => void preview());
</script>

<div class="provider-resource-details">
  <div class="provider-resource-meta">
    <div><span>仓库类型</span><strong>{resource.subtype ?? '—'}</strong></div>
    <div>
      <span>资源端点</span><strong>{resourceEndpointFor(resource)}</strong>
    </div>
    <div>
      <span>当前分支</span><strong
        >{savedRevision.branch ||
          String(resource.config?.default_branch ?? '—')}</strong
      >
    </div>
    <div>
      <span>当前 CommitID</span><strong>{savedRevision.commitID || '—'}</strong>
    </div>
    {#if isBundle}
      <div>
        <span>存储后端</span><strong
          >{String(resource.config?.storage_backend ?? '—')}</strong
        >
      </div>
      <div>
        <span>存储标识</span><strong
          >{String(resource.config?.storage_key ?? '—')}</strong
        >
      </div>
      <div>
        <span>Bundle 状态</span><strong
          >{resource.config?.storage_key ? '已上传' : '未上传'}</strong
        >
      </div>
    {:else}
      <div>
        <span>仓库地址</span><strong
          >{String(resource.config?.url ?? '—')}</strong
        >
      </div>
      <div>
        <span>连接凭据</span><strong
          >{resource.credential_configured ? '已配置' : '未配置'}</strong
        >
      </div>
    {/if}
    <div>
      <span>最新更新</span><strong>{formatDate(resource.updated_at)}</strong>
    </div>
  </div>
</div>
<RepositoryPreviewContent
  {branches}
  bind:selectedBranch
  bind:selectedCommit
  {busy}
  {failed}
  {failureReason}
  entries={previewEntries}
  revisionEditable={canUpdate}
  showRefresh
  subtitle="切换版本仅用于预览，不修改资源配置"
  on:refresh={() => void preview()}
  on:revisionchange={selectRevision}
/>
