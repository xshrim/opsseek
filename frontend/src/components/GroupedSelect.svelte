<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { ChevronDown, ChevronRight } from 'lucide-svelte';

  export let groups: {
    value: string;
    label: string;
    options: { value: string; label: string; description?: string }[];
  }[] = [];
  export let group = '';
  export let value = '';
  export let groupLabel = '分组';
  export let optionLabel = '选项';
  export let ariaLabel = '选择';
  export let placeholder = '请选择';
  export let tooltip = '';
  export let disabled = false;
  export let placement: 'top' | 'bottom' = 'bottom';
  export let menuWidth = '';
  const dispatch = createEventDispatcher<{
    change: { group: string; value: string };
  }>();
  let open = false;
  let activeGroup = '';
  let root: HTMLDivElement;
  let trigger: HTMLButtonElement;
  $: selectedGroup = groups.find((item) => item.value === group);
  $: selectedOption = selectedGroup?.options.find(
    (item) => item.value === value
  );
  $: menuGroup = groups.find((item) => item.value === activeGroup);
  $: selectedTooltip =
    selectedGroup && value ? `${selectedGroup.label} · ${value}` : placeholder;

  function toggle() {
    open = !open;
    if (open) activeGroup = group || groups[0]?.value || '';
  }
  function choose(next: string) {
    group = activeGroup;
    value = next;
    open = false;
    dispatch('change', { group, value });
    trigger?.focus();
  }
  function outside(event: PointerEvent) {
    if (root && !root.contains(event.target as Node)) open = false;
  }
</script>

<svelte:window
  on:pointerdown={outside}
  on:keydown={(event) => {
    if (open && event.key === 'Escape') {
      open = false;
      trigger?.focus();
    }
  }}
/>
<div
  class="grouped-select"
  bind:this={root}
  style={`--grouped-select-menu-width:${menuWidth || '100%'}`}
>
  <button
    class="grouped-select-trigger"
    bind:this={trigger}
    type="button"
    disabled={disabled || !groups.length}
    aria-label={ariaLabel}
    aria-haspopup="dialog"
    aria-expanded={open}
    data-tooltip={tooltip || selectedTooltip}
    on:click={toggle}
  >
    <span
      >{selectedGroup
        ? `${selectedGroup.label} · ${selectedOption?.label ?? value}`
        : placeholder}</span
    >
    <ChevronDown size={13} aria-hidden="true" />
  </button>
  {#if open}
    <div
      class="grouped-select-menu"
      class:above={placement === 'top'}
      role="dialog"
      aria-label={ariaLabel}
    >
      <div class="group-list" role="group" aria-label={groupLabel}>
        <small>{groupLabel}</small>
        {#each groups as item (item.value)}
          <button
            type="button"
            class:active={activeGroup === item.value}
            aria-pressed={activeGroup === item.value}
            on:click={() => (activeGroup = item.value)}
          >
            <span data-tooltip={item.label}>{item.label}</span><ChevronRight
              size={12}
              aria-hidden="true"
            />
          </button>
        {/each}
      </div>
      <div class="option-list" role="group" aria-label={optionLabel}>
        <small>{optionLabel}</small>
        {#each menuGroup?.options ?? [] as item (item.value)}
          <button
            type="button"
            class:active={group === activeGroup && value === item.value}
            aria-pressed={group === activeGroup && value === item.value}
            data-tooltip={item.value}
            on:click={() => choose(item.value)}
          >
            <span
              >{item.label}{#if item.description}<small
                  >{item.description}</small
                >{/if}</span
            >
          </button>
        {:else}<small>暂无选项</small>{/each}
      </div>
    </div>
  {/if}
</div>

<style>
  .grouped-select {
    position: relative;
    min-width: 0;
  }
  .grouped-select-trigger {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    max-width: none;
    min-height: 32px;
    padding: 0 8px;
    color: var(--theme-fg-muted);
    background: transparent;
    border: 1px solid var(--theme-border);
    border-radius: 4px;
    font-size: 11px;
    text-align: left;
  }
  .grouped-select-trigger > span {
    min-width: 0;
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .grouped-select-trigger :global(svg) {
    flex: 0 0 auto;
  }
  .grouped-select-trigger:disabled {
    cursor: not-allowed;
    opacity: 0.65;
  }
  .grouped-select-menu {
    box-sizing: border-box;
    position: absolute;
    z-index: 1000;
    left: 0;
    right: auto;
    top: calc(100% + 6px);
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1.5fr);
    width: min(var(--grouped-select-menu-width), calc(100vw - 16px));
    min-width: 100%;
    overflow: hidden;
    padding: 5px;
    color: var(--theme-fg);
    background: var(--theme-bg-raised);
    border: 1px solid var(--theme-border-strong);
    border-radius: 5px;
    box-shadow: var(--theme-shadow);
  }
  .grouped-select-menu.above {
    top: auto;
    bottom: calc(100% + 6px);
  }
  .group-list,
  .option-list {
    box-sizing: border-box;
    min-width: 0;
    max-height: 280px;
    overflow: auto;
    overflow-x: hidden;
    padding: 2px;
  }
  .option-list {
    border-left: 1px solid var(--theme-border);
  }
  small {
    display: block;
    padding: 3px 6px 5px;
    color: var(--theme-fg-muted);
    font-size: 10px;
  }
  .grouped-select-menu button {
    box-sizing: border-box;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 6px;
    width: 100%;
    min-height: 28px;
    padding: 5px 6px;
    color: var(--theme-fg-muted);
    background: transparent;
    border: 0;
    border-radius: 3px;
    font-size: 11px;
    text-align: left;
  }
  .grouped-select-menu button span {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .grouped-select-menu button small {
    padding: 2px 0 0;
  }
  .grouped-select-menu button:hover,
  .grouped-select-menu button.active {
    color: var(--theme-action);
    background: var(--theme-bg-selected);
  }
</style>
