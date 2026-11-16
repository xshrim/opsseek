<script context="module" lang="ts">
  let nextComboboxId = 0;
  export type ComboboxOption = {
    value: string;
    label: string;
    description?: string;
    iconKind?: string;
    icon?: string;
    iconClassName?: string;
  };
</script>

<script lang="ts">
  import { createEventDispatcher, tick } from 'svelte';
  import { AppWindow } from 'lucide-svelte';
  import EntityBrandIcon from './EntityBrandIcon.svelte';

  export let options: ComboboxOption[] = [];
  export let value = '';
  export let placeholder = '选择或输入';
  export let ariaLabel = placeholder;
  export let disabled = false;
  export let allowEmpty = false;

  const dispatch = createEventDispatcher<{ change: string }>();
  const listId = `combobox-options-${++nextComboboxId}`;
  let root: HTMLDivElement;
  let input: HTMLInputElement;
  let open = false;
  let activeIndex = -1;
  let query = '';
  let menuStyle = '';

  $: normalizedQuery = query.trim().toLocaleLowerCase();
  $: filteredOptions = normalizedQuery
    ? options.filter((option) =>
        `${option.label} ${option.description ?? ''}`
          .toLocaleLowerCase()
          .includes(normalizedQuery)
      )
    : options;
  $: displayValue =
    query || options.find((option) => option.value === value)?.label || value;

  function updateValue(next: string) {
    value = next;
    dispatch('change', next);
  }

  function handleInput(event: Event) {
    const next = (event.currentTarget as HTMLInputElement).value;
    query = next;
    activeIndex = -1;
    updateValue(next);
    void positionMenu();
  }

  async function positionMenu() {
    open = true;
    await tick();
    if (!input) return;
    const rect = input.getBoundingClientRect();
    menuStyle = `position:fixed;top:${rect.bottom + 4}px;left:${rect.left}px;width:${rect.width}px;z-index:60`;
  }

  function choose(option: ComboboxOption) {
    query = '';
    updateValue(option.value);
    open = false;
    menuStyle = '';
  }

  function onKeydown(event: KeyboardEvent) {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      void positionMenu();
      const direction = event.key === 'ArrowDown' ? 1 : -1;
      activeIndex = Math.max(
        0,
        Math.min(filteredOptions.length - 1, activeIndex + direction)
      );
    } else if (event.key === 'Enter') {
      event.preventDefault();
      const option = filteredOptions[activeIndex];
      if (open && option) choose(option);
      else open = false;
    } else if (event.key === 'Escape') {
      open = false;
      menuStyle = '';
      activeIndex = -1;
    }
  }

  function closeOutside(event: MouseEvent) {
    if (root && !root.contains(event.target as Node)) {
      open = false;
      activeIndex = -1;
      menuStyle = '';
      query = '';
    }
  }

  function clear() {
    query = '';
    updateValue('');
    input?.focus();
    void positionMenu();
  }
</script>

<svelte:window on:click={closeOutside} />

<div class="combobox" bind:this={root}>
  <input
    bind:this={input}
    type="text"
    value={displayValue}
    {placeholder}
    {disabled}
    aria-label={ariaLabel}
    role="combobox"
    aria-autocomplete="list"
    aria-expanded={open}
    aria-controls={listId}
    on:focus={() => void positionMenu()}
    on:input={handleInput}
    on:keydown={onKeydown}
  />
  {#if value && !disabled}<button
      class="combobox-clear"
      type="button"
      aria-label="清空"
      on:click|preventDefault|stopPropagation={clear}>×</button
    >{/if}
  {#if open && !disabled}<div
      class="combobox-menu"
      style={menuStyle}
      role="listbox"
      id={listId}
      aria-label={ariaLabel}
    >
      {#if allowEmpty && !normalizedQuery}<button
          class="combobox-option combobox-empty-option"
          type="button"
          role="option"
          aria-selected={!value}
          on:click={() => choose({ value: '', label: '' })}
          >不选择</button
        >{/if}
      {#each filteredOptions as option, index (option.value)}
        <button
          class="combobox-option"
          class:active={activeIndex === index}
          type="button"
          role="option"
          aria-selected={value === option.value}
          on:mouseenter={() => (activeIndex = index)}
          on:click={() => choose(option)}
        >
          {#if option.iconKind === 'Application'}<AppWindow
              class="project-application-node-icon"
              size={16}
            />{:else if option.iconKind}<EntityBrandIcon
              kind={option.iconKind}
              fallback={option.icon || 'lucide:Circle'}
              className={`${option.iconClassName || ''} project-entity-tone`}
              size={16}
            />{/if}
          <span><strong>{option.label}</strong>{#if option.description}<small
                >{option.description}</small
              >{/if}</span
        ></button>
      {:else}
        <div class="combobox-no-options">
          {normalizedQuery ? '使用输入内容' : '暂无选项'}
        </div>
      {/each}
    </div>{/if}
</div>

<style>
  .combobox {
    position: relative;
    min-width: 0;
  }
  input {
    width: 100%;
    min-width: 0;
    min-height: 32px;
    padding: 6px 28px 6px 8px;
    color: var(--theme-fg);
    background: var(--theme-bg-input);
    border: 1px solid var(--theme-border-strong);
    border-radius: 4px;
    outline: 0;
    font-size: 11px;
  }
  input:focus-visible {
    border-color: var(--theme-border-focus);
    box-shadow: 0 0 0 2px var(--theme-focus);
  }
  .combobox-clear {
    position: absolute;
    top: 2px;
    right: 3px;
    width: 25px;
    height: 27px;
    padding: 0;
    color: var(--theme-fg-muted);
    background: transparent;
    border: 0;
  }
  .combobox-menu {
    position: absolute;
    z-index: 40;
    top: 0;
    left: 0;
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    min-width: 0;
    max-width: min(360px, calc(100vw - 24px));
    box-sizing: border-box;
    max-height: 240px;
    overflow: auto;
    padding: 4px;
    background: var(--theme-bg-raised);
    border: 1px solid var(--theme-border-strong);
    border-radius: 5px;
    box-shadow: var(--theme-shadow-soft);
  }
  .combobox-option {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    box-sizing: border-box;
    min-width: 0;
    padding: 6px 8px;
    color: var(--theme-fg);
    text-align: left;
    background: transparent;
    border: 0;
    border-radius: 3px;
    font-size: 11px;
    white-space: nowrap;
  }
  .combobox-option:hover,
  .combobox-option.active {
    background: var(--theme-bg-selected);
  }
  .combobox-option > span {
    display: grid;
    min-width: 0;
    gap: 2px;
  }
  .combobox-option strong,
  .combobox-option small {
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .combobox-option small,
  .combobox-no-options {
    color: var(--theme-fg-muted);
    font-size: 10px;
  }
  .combobox-empty-option,
  .combobox-no-options {
    padding: 7px 8px;
  }
</style>
