<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte';
  import { Eye } from 'lucide-svelte';
  import FormField from '../../components/FormField.svelte';
  import GroupedSelect from '../../components/GroupedSelect.svelte';
  import PasswordInput from '../../components/PasswordInput.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import type { RepositoryBranchRevision } from './repositoryPreview';
  import { repositoryRevisionGroups } from './repositoryPreview';

  export let subtype = 'Git';
  export let url = '';
  export let username = '';
  export let password = '';
  export let credentialConfigured = false;
  export let editing = false;
  export let bundleConfigured = false;
  export let defaultBranch = 'main';
  export let branches: RepositoryBranchRevision[] = [];
  export let selectedBranch = '';
  export let selectedCommit = '';
  export let loadingRevisions = false;
  export let previewBusy = false;
  export let configurationAttempted = false;
  const dispatch = createEventDispatcher<{
    input: void;
    revisionsneeded: void;
    revisionchange: { branch: string; commitID: string };
    preview: void;
  }>();
  $: issues =
    subtype === 'Git'
      ? [
          !url.trim() ? 'Git 仓库 URL' : '',
          !selectedBranch || !selectedCommit ? '默认分支和 Commit ID' : ''
        ].filter(Boolean)
      : !bundleConfigured
        ? ['Bundle 文件']
        : [];
  $: valid = issues.length === 0;
  $: revisionGroups = repositoryRevisionGroups(
    branches,
    selectedBranch,
    selectedCommit
  );

  onMount(() => {
    if (subtype === 'Git' && url.trim()) dispatch('revisionsneeded');
  });
</script>

{#if subtype === 'Git'}
  <div class="git-config-row">
    <FormField className="git-url-field" label="Git 仓库 URL" required>
      <TextInput
        bind:value={url}
        type="url"
        placeholder="https://git.example.com/team/repo.git"
        on:input={() => dispatch('input')}
      />
    </FormField>
    <div class="git-credential-fields">
      <FormField label="HTTPS 用户名">
        <TextInput
          bind:value={username}
          autocomplete="username"
          placeholder={editing && credentialConfigured
            ? '留空以保留现有凭据'
            : ''}
          on:input={() => dispatch('input')}
        />
      </FormField>
      <FormField label="HTTPS 访问令牌">
        <PasswordInput
          bind:value={password}
          autocomplete="new-password"
          placeholder={editing && credentialConfigured
            ? '留空以保留现有凭据'
            : ''}
          ariaLabel="HTTPS 访问令牌"
          secretLabel="访问令牌"
          on:input={() => dispatch('input')}
        />
      </FormField>
    </div>
    <div class="git-revision-controls">
      <FormField label="默认分支" required>
        <GroupedSelect
          groups={revisionGroups}
          group={selectedBranch || defaultBranch}
          value={selectedCommit}
          groupLabel="分支"
          optionLabel="Commit ID"
          ariaLabel="选择默认分支和 Commit ID"
          placeholder={loadingRevisions
            ? '正在读取版本…'
            : '填写仓库地址以读取版本'}
          disabled={loadingRevisions || branches.length === 0}
          on:change={(event) =>
            dispatch('revisionchange', {
              branch: event.detail.group,
              commitID: event.detail.value
            })}
        />
      </FormField>
      <button
        class="secondary repository-preview-button"
        type="button"
        disabled={!url.trim() ||
          !selectedBranch ||
          !selectedCommit ||
          loadingRevisions ||
          previewBusy}
        on:click={() => dispatch('preview')}
      >
        <Eye size={14} aria-hidden="true" />{previewBusy ? '预览中…' : '预览'}
      </button>
    </div>
    {#if configurationAttempted && !valid}<p
        class="form-error git-config-error"
      >
        请填写 Git 仓库 URL 并选择默认分支和 Commit ID。
      </p>{/if}
  </div>
{:else if configurationAttempted && !valid}
  <p class="form-error">请上传 Bundle 文件。</p>
{/if}

<style>
  .git-config-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) minmax(0, 1fr);
    align-items: end;
    gap: 12px;
    min-width: 0;
  }
  .git-credential-fields {
    display: grid;
    gap: 7px;
    min-width: 0;
  }
  .git-revision-controls {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: end;
    gap: 7px;
    min-width: 0;
  }
  .repository-preview-button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 5px;
    min-height: 36px;
    white-space: nowrap;
  }
  .git-config-error {
    grid-column: 1 / -1;
    margin: 0;
  }
  @media (max-width: 760px) {
    .git-config-row {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
