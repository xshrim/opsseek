<script lang="ts">
  import { Check, Copy, X } from 'lucide-svelte';
  import Dialog from '../../components/Dialog.svelte';
  import type { Resource } from '../../lib/api';

  export let resource: Resource | null = null;
  export let workflow = false;
  let open = false;
  let copied = '';
  $: resourceID = resource?.id ?? '<资源 ID>';
  $: apiBase = 'https://opsseek.example.com';
  $: apiUpload = [
    'curl -fsS -b "$COOKIE_FILE" \\',
    "  -F 'bundle=@opsseek.bundle' \\",
    '  "' + apiBase + '/api/v1/resources/' + resourceID + '/bundle"'
  ].join('\n');

  async function copyCommand() {
    await navigator.clipboard.writeText(apiUpload);
    copied = 'api';
    window.setTimeout(() => (copied = ''), 1600);
  }
</script>

<button class="guide-link" type="button" on:click={() => (open = true)}
  >该如何做</button
>

{#if open}
  <Dialog
    className="repository-guide-backdrop"
    dialogClass="repository-guide-dialog"
    modal={false}
    label="如何生成并上传 Git Bundle"
    closeOnBackdrop
    resizable={false}
    onBackdrop={() => (open = false)}
  >
    <section class="guide-modal">
      <header>
        <div>
          <h2>如何生成并上传 Git Bundle</h2>
          <p>Bundle 会包含仓库历史，上传后可选择分支和 commit。</p>
        </div>
        <button
          class="close-button"
          type="button"
          aria-label="关闭"
          on:click={() => (open = false)}
          ><X size={17} aria-hidden="true" /></button
        >
      </header>
      <ol>
        <li>
          <strong>在仓库工作副本中生成并验证</strong>
          <pre><code
              >git bundle create opsseek.bundle --all
git bundle verify opsseek.bundle</code
            ></pre>
        </li>
        <li>
          <strong>选择文件并点击上传</strong>
          <p>
            上传成功后，页面会显示仓库目录预览。编辑已有资源时，不选新文件即可保留当前
            Bundle。
          </p>
        </li>
      </ol>
      {#if !workflow}
        <div class="api-heading">
          <strong>也可通过 API 上传</strong><button
            type="button"
            class="copy-button"
            aria-label="复制上传命令"
            data-tooltip="复制上传命令"
            on:click={() => void copyCommand()}
            >{#if copied === 'api'}<Check
                size={15}
                aria-hidden="true"
              />{:else}<Copy size={15} aria-hidden="true" />{/if}</button
          >
        </div>
        <p>
          将地址、资源 ID 和 Cookie 替换为实际值；账号需有此资源的更新权限。
        </p>
        <pre><code>{apiUpload}</code></pre>
      {/if}
    </section>
  </Dialog>
{/if}

<style>
  .guide-link {
    padding: 2px 0;
    border: 0;
    background: transparent;
    color: var(--theme-action);
    font-size: 11px;
    cursor: pointer;
    text-decoration: underline;
    text-underline-offset: 3px;
    white-space: nowrap;
  }
  :global(.dialog.repository-guide-dialog) {
    width: min(620px, calc(100vw - 36px));
    max-height: min(80vh, 720px);
    padding: 0;
    overflow: auto;
    border: 1px solid var(--theme-border);
    border-radius: 8px;
    background: var(--theme-bg-surface);
    color: var(--theme-fg);
    box-shadow: var(--theme-shadow);
  }
  .guide-modal {
    display: grid;
    gap: 14px;
    padding: 18px;
  }
  .guide-modal header,
  .api-heading {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
  }
  .guide-modal h2 {
    margin: 0;
    color: var(--theme-fg-strong);
    font-size: 15px;
  }
  .guide-modal p {
    margin: 5px 0 0;
    color: var(--theme-fg-muted);
    font-size: 12px;
    line-height: 1.55;
  }
  .close-button,
  .copy-button {
    display: grid;
    width: 30px;
    height: 30px;
    flex: 0 0 auto;
    place-items: center;
    border: 1px solid var(--theme-border);
    border-radius: 5px;
    background: var(--theme-bg-raised);
    color: var(--theme-fg-muted);
    cursor: pointer;
  }
  .guide-modal ol {
    display: grid;
    gap: 14px;
    margin: 0;
    padding-left: 22px;
  }
  .guide-modal li::marker {
    color: var(--theme-action);
    font-weight: 700;
  }
  .guide-modal li > strong,
  .api-heading strong {
    color: var(--theme-fg-strong);
    font-size: 12px;
  }
  .guide-modal pre {
    overflow: auto;
    margin: 8px 0 0;
    padding: 10px 12px;
    border: 1px solid var(--theme-border);
    border-radius: 5px;
    background: var(--theme-bg-subtle);
    color: var(--theme-fg-strong);
    font:
      11px/1.6 ui-monospace,
      SFMono-Regular,
      Menlo,
      monospace;
    white-space: pre;
  }
</style>
