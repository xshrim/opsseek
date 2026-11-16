<script lang="ts">
  import { Upload } from 'lucide-svelte';
  import { createEventDispatcher } from 'svelte';
  import { formatFileSize } from '../lib/format';

  export let file: File | null = null;
  export let accept = '';
  export let ariaLabel = '选择上传文件';
  export let placeholder = '选择或拖入文件';
  export let className = '';
  export let showUploadButton = false;
  export let uploading = false;
  export let disabled = false;
  const dispatch = createEventDispatcher<{
    change: File | null;
    upload: File;
  }>();

  function selectFile(event: Event) {
    file = (event.currentTarget as HTMLInputElement).files?.[0] ?? null;
    dispatch('change', file);
  }

  function upload() {
    if (file && !uploading && !disabled) dispatch('upload', file);
  }
</script>

<div class={`bundle-file-input ${className}`.trim()}>
  <div class="bundle-file-control" class:with-upload={showUploadButton}>
    <input
      type="file"
      {accept}
      aria-label={ariaLabel}
      on:change={selectFile}
      disabled={disabled || uploading}
    />
    <Upload class="upload-icon" size={17} aria-hidden="true" />
    <span class="file-copy" title={file?.name ?? ariaLabel}>
      <strong>{file?.name ?? placeholder}</strong>
      {#if file}<small>{formatFileSize(file.size)}</small>{/if}
    </span>
    {#if showUploadButton}
      <button
        type="button"
        class="upload-button"
        disabled={!file || uploading || disabled}
        on:click={upload}
      >
        {uploading ? '上传中…' : '上传'}
      </button>
    {/if}
  </div>
</div>

<style>
  .bundle-file-input {
    width: 100%;
    min-width: 0;
  }
  .bundle-file-control {
    position: relative;
    display: flex;
    align-items: center;
    gap: 9px;
    width: 100%;
    min-width: 0;
    min-height: 42px;
    padding: 5px 6px 5px 11px;
    color: var(--theme-fg-muted);
    background: var(--theme-bg-input);
    border: 1px solid var(--theme-border-strong);
    border-radius: 6px;
    box-sizing: border-box;
    transition:
      border-color 120ms ease,
      box-shadow 120ms ease,
      background 120ms ease;
  }
  .bundle-file-control:hover {
    background: var(--theme-bg-raised);
    border-color: var(--theme-border-focus);
  }
  .bundle-file-control:focus-within {
    border-color: var(--theme-border-focus);
    box-shadow: 0 0 0 3px var(--theme-focus);
  }
  .upload-icon {
    flex: 0 0 auto;
    color: var(--theme-accent);
  }
  .file-copy {
    display: grid;
    flex: 1 1 auto;
    gap: 2px;
    min-width: 0;
  }
  .file-copy strong {
    overflow: hidden;
    color: var(--theme-fg-strong);
    font-size: 12px;
    font-weight: 500;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .file-copy small {
    color: var(--theme-fg-muted);
    font-size: 10px;
  }
  .bundle-file-control input {
    position: absolute;
    z-index: 1;
    top: 0;
    bottom: 0;
    left: 0;
    width: 100%;
    height: 100%;
    opacity: 0;
    cursor: pointer;
  }
  .bundle-file-control input:disabled {
    cursor: not-allowed;
  }
  .upload-button {
    z-index: 2;
    display: inline-flex;
    min-height: 32px;
    flex: 0 0 auto;
    align-items: center;
    justify-content: center;
    padding: 0 13px;
    border: 0;
    border-radius: 4px;
    background: var(--color-primary);
    color: var(--theme-accent-contrast);
    font-size: 12px;
    font-weight: 650;
    cursor: pointer;
  }
  .upload-button:disabled {
    cursor: not-allowed;
    opacity: 0.55;
  }
  .bundle-file-control:not(.with-upload) input {
    width: 100%;
  }
</style>
