<script lang="ts">
  import { Check, ListPlus } from 'lucide-svelte';
  import { tick } from 'svelte';
  import ResourceBrandIcon from '../../components/ResourceBrandIcon.svelte';
  import ScopeLevelBadge from '../../components/ScopeLevelBadge.svelte';
  import SearchInput from '../../components/SearchInput.svelte';
  import type { Resource } from '../../lib/api';
  import {
    resourceCatalogTagClass,
    resourceCatalogTagsFor,
    resourceSubtypeFor
  } from '../resource/resourceCatalog';

  export let resources: Resource[] = [];
  export let resourceScopeTypes: Record<string, string> = {};
  export let selectedResourceId = '';
  export let unavailableResourceIds: string[] = [];
  export let onSelect: (resourceId: string) => void = () => {};

  let root: HTMLDivElement;
  let open = false;
  let query = '';
  let menuStyle = '';

  $: selectedResource = resources.find(
    (resource) => resource.id === selectedResourceId
  );
  $: normalizedQuery = query.trim().toLocaleLowerCase();
  $: options = resources.filter((resource) => {
    if (
      resource.id !== selectedResourceId &&
      unavailableResourceIds.includes(resource.id)
    )
      return false;
    if (!normalizedQuery) return true;
    return [
      resource.name,
      resource.kind,
      resourceSubtypeFor(resource),
      ...resourceCatalogTagsFor(resource)
    ].some((value) => value.toLocaleLowerCase().includes(normalizedQuery));
  });
  $: scopeType = selectedResource
    ? normalizedScopeType(resourceScopeTypes[selectedResource.scope_id])
    : 'scope';

  async function positionMenu() {
    await tick();
    if (!open || !root) return;
    const rect = root.getBoundingClientRect();
    const gap = 5;
    const padding = 8;
    const width = Math.min(Math.max(rect.width, 320), window.innerWidth - 16);
    const left = Math.max(
      8,
      Math.min(rect.left, window.innerWidth - width - 8)
    );
    const below = window.innerHeight - rect.bottom - gap - padding;
    const above = rect.top - gap - padding;
    if (below >= Math.min(300, above) || below >= 220) {
      menuStyle = `top:${Math.round(rect.bottom + gap)}px;left:${Math.round(left)}px;width:${Math.round(width)}px;max-height:${Math.round(Math.max(180, below))}px;`;
    } else {
      menuStyle = `bottom:${Math.round(window.innerHeight - rect.top + gap)}px;left:${Math.round(left)}px;width:${Math.round(width)}px;max-height:${Math.round(Math.max(180, above))}px;`;
    }
  }

  function toggle() {
    open = !open;
    query = '';
    if (open) void positionMenu();
    else menuStyle = '';
  }

  function select(resourceId: string) {
    onSelect(resourceId);
    open = false;
    query = '';
    menuStyle = '';
  }

  function close(event: MouseEvent) {
    if (root && !root.contains(event.target as Node)) {
      open = false;
      menuStyle = '';
    }
  }

  function handleViewportChange() {
    if (open) void positionMenu();
  }

  function normalizedScopeType(value = ''): 'platform' | 'team' | 'project' | 'scope' {
    return value === 'platform' || value === 'team' || value === 'project'
      ? value
      : 'scope';
  }

  function scopeLabel(value = '') {
    if (value === 'platform') return '平台';
    if (value === 'team') return '团队';
    if (value === 'project') return '项目';
    return '其他范围';
  }
</script>

<svelte:window
  on:click={close}
  on:resize={handleViewportChange}
  on:scroll={handleViewportChange}
/>

<div class="resource-binding-picker" bind:this={root}>
  <button
    class="resource-binding-picker-trigger"
    type="button"
    aria-label={selectedResource
      ? `更换关联资源：${selectedResource.name}`
      : '选择关联资源'}
    aria-haspopup="listbox"
    aria-expanded={open}
    data-tooltip={selectedResource ? '更换关联资源' : '选择关联资源'}
    on:click|preventDefault|stopPropagation={toggle}
    ><ListPlus size={17} /></button
  >
  {#if open}
    <div
      class="resource-binding-picker-menu"
      style={menuStyle}
      role="listbox"
      aria-label="选择关联资源"
    >
      <SearchInput
        bind:value={query}
        width="100%"
        height="32px"
        placeholder="搜索资源"
        ariaLabel="搜索关联资源"
      />
      <div class="resource-binding-picker-options">
        {#each options as resource (resource.id)}
          {@const tag = resourceCatalogTagsFor(resource)[0] ?? '资源'}
          {@const tagClass = resourceCatalogTagClass(tag)}
          {@const itemScopeType = normalizedScopeType(
            resourceScopeTypes[resource.scope_id]
          )}
          <button
            class="resource-binding-picker-option"
            class:runtime-binding-option={['Host', 'Docker', 'Kubernetes'].includes(resource.kind)}
            class:resource-binding-option={!['Host', 'Docker', 'Kubernetes'].includes(resource.kind)}
            class:selected={resource.id === selectedResourceId}
            type="button"
            role="option"
            aria-selected={resource.id === selectedResourceId}
            on:click|stopPropagation={() => select(resource.id)}
          >
            <span
              class:project-runtime-icon={['Host', 'Docker', 'Kubernetes'].includes(resource.kind)}
              class:project-resource-icon={!['Host', 'Docker', 'Kubernetes'].includes(resource.kind)}
            ><ResourceBrandIcon {resource} size={17} /></span>
            <span class="resource-binding-picker-option-copy">
              <span class="resource-binding-picker-option-title">
                <strong>{resource.name}</strong>
                <span class={`resource-catalog-tag resource-catalog-tag-${tagClass}`}
                  >{tag}</span
                >
              </span>
            <small>{resource.kind} · {resourceSubtypeFor(resource)}</small>
            </span>
            <ScopeLevelBadge
              className="resource-binding-level-badge"
              type={itemScopeType}
              label={scopeLabel(resourceScopeTypes[resource.scope_id])}
            />
            {#if resource.id === selectedResourceId}<Check
                size={14}
                class="resource-binding-picker-check"
              />{/if}
          </button>
        {:else}<div class="resource-binding-picker-empty">没有可选资源</div>{/each}
      </div>
    </div>
  {/if}
</div>

<style>
  .resource-binding-picker {
    position: relative;
    display: grid;
    flex: 0 0 auto;
    place-items: center;
    width: 28px;
    height: 28px;
  }
  .resource-binding-picker-trigger {
    display: grid;
    place-items: center;
    width: 27px;
    height: 27px;
    padding: 0;
    color: var(--theme-fg-muted);
    background: transparent;
    border: 0;
    border-radius: 4px;
  }
  .resource-binding-picker-trigger:hover,
  .resource-binding-picker-trigger:focus-visible,
  .resource-binding-picker-trigger[aria-expanded='true'] {
    color: var(--theme-action);
    background: var(--theme-bg-selected);
  }
  .resource-binding-picker-menu {
    position: fixed;
    z-index: 1100;
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
    gap: 5px;
    padding: 6px;
    overflow: hidden;
    background: var(--theme-bg-raised);
    border: 1px solid var(--theme-border-strong);
    border-radius: 5px;
    box-shadow: var(--theme-shadow);
  }
  .resource-binding-picker-options {
    min-height: 0;
    overflow: auto;
  }
  .resource-binding-picker-option {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    min-height: 48px;
    padding: 6px 7px;
    color: var(--theme-fg);
    text-align: left;
    background: transparent;
    border: 0;
    border-radius: 4px;
  }
  .resource-binding-picker-option:hover,
  .resource-binding-picker-option.selected {
    background: var(--theme-bg-selected);
  }
  .resource-binding-picker-option-copy {
    display: grid;
    flex: 1;
    gap: 3px;
    min-width: 0;
  }
  .resource-binding-picker-option-title {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
  }
  .resource-binding-picker-option-title strong {
    overflow: hidden;
    font-size: 11px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .resource-binding-picker-option-copy small {
    overflow: hidden;
    color: var(--theme-fg-muted);
    font-size: 10px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .resource-binding-level-badge {
    flex: 0 0 auto;
  }
  .resource-binding-picker-check {
    flex: 0 0 auto;
    color: var(--theme-action);
  }
  .resource-binding-picker-empty {
    padding: 15px 8px;
    color: var(--theme-fg-muted);
    text-align: center;
    font-size: 11px;
  }
</style>
