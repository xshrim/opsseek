<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import type { Resource } from '../../lib/api';
  import { appURL } from '../../lib/health';
  import FileUpload from '../../components/FileUpload.svelte';
  import RepositoryPreviewContent from './RepositoryPreviewContent.svelte';
  import type {
    RepositoryBranchRevision,
    RepositoryPreviewEntry
  } from './repositoryPreview';

  export let resource: Resource;
  export let canUpdate = false;
  let busy = false;
  let failed = false;
  let failureReason = '';
  let uploading = false;
  let file: File | null = null;
  let previewEntries: RepositoryPreviewEntry[] = [];
  let branches: RepositoryBranchRevision[] = [];
  let selectedBranch = '';
  let selectedCommit = '';
  const dispatch = createEventDispatcher<{ resourceupdate: Resource }>();
  $: isBundle = String(resource.subtype ?? '').toLowerCase() === 'bundle';

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

  async function uploadBundle(event: CustomEvent<File>) {
    if (uploading || !event.detail) return;
    uploading = true;
    failed = false;
    failureReason = '';
    let uploaded = false;
    try {
      const body = new FormData();
      body.append('bundle', event.detail);
      const response = await fetch(
        appURL('api/v1/resources/' + resource.id + '/bundle'),
        {
          method: 'POST',
          body,
          credentials: 'include'
        }
      );
      const data = await response.json().catch(() => ({}));
      if (!response.ok)
        throw new Error(data?.error?.message ?? data?.message ?? '上传失败');
      const branch = String(data.default_branch ?? '');
      const commitID = String(data.commit_id ?? '');
      const storageKey = String(data.storage_key ?? '');
      const storageBackend = String(data.storage_backend ?? '');
      if (!branch || !commitID || !storageKey || !storageBackend)
        throw new Error('上传成功，但未返回所选分支或 commit。');
      resource = {
        ...resource,
        config: {
          ...resource.config,
          storage_key: storageKey,
          storage_backend: storageBackend,
          default_branch: branch,
          endpoint: resource.id + ' [' + branch + '·' + commitID + ']'
        }
      };
      dispatch('resourceupdate', resource);
      uploaded = true;
    } catch (error) {
      failed = true;
      failureReason =
        error instanceof Error
          ? error.message
          : 'Bundle 上传失败，请稍后重试。';
    } finally {
      uploading = false;
    }
    if (uploaded) await preview('', '');
  }

  onMount(() => void preview());
</script>

{#if isBundle && canUpdate}
  <FileUpload
    bind:file
    accept=".bundle,application/octet-stream"
    ariaLabel="选择 Git Bundle 文件"
    placeholder="选择或拖入 Bundle 文件"
    showUploadButton
    {uploading}
    disabled={busy}
    on:upload={uploadBundle}
  />
{/if}
<RepositoryPreviewContent
  {branches}
  bind:selectedBranch
  bind:selectedCommit
  busy={busy || uploading}
  {failed}
  {failureReason}
  entries={previewEntries}
  revisionEditable={canUpdate}
  showRefresh
  subtitle="切换版本仅用于预览，不修改资源配置"
  on:refresh={() => void preview()}
  on:revisionchange={selectRevision}
/>
