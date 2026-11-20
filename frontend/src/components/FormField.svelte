<script lang="ts">
  import FieldLabel from './FieldLabel.svelte';
  export let label = '';
  export let required = false;
  export let invalid = false;
  export let error = '';
  export let invalidMessage = '请检查此项填写内容。';
  export let errorId = '';
  export let help = '';
  export let className = '';
  export let asLabel = true;
</script>

<svelte:element
  this={asLabel ? 'label' : 'div'}
  class={`form-field ${className}`.trim()}
  class:invalid={invalid || Boolean(error)}
  data-error={error || (invalid ? invalidMessage : undefined)}
>
  <FieldLabel
    text={label}
    {required}
    invalid={invalid || Boolean(error)}
    className="form-field-label"
  />
  <slot />
  {#if error || invalid}<small id={errorId || undefined} class="form-field-error" role="alert">{error || invalidMessage}</small
    >{:else if help}<small class="form-field-message">{help}</small>{/if}
</svelte:element>

<style>
  .form-field {
    position: relative;
    display: grid;
    gap: 6px;
    min-width: 0;
    color: var(--theme-fg-muted);
    font-size: 12px;
    font-weight: 400;
  }
  .form-field-label {
    display: block;
  }
  .form-field-message {
    color: var(--theme-fg-muted);
    font-size: 11px;
    font-weight: 400;
    line-height: 1.35;
  }
  .form-field-error {
    position: absolute;
    z-index: 40;
    top: calc(100% + 6px);
    left: 0;
    max-width: min(320px, 90vw);
    padding: 6px 9px;
    color: var(--theme-danger);
    background: var(--theme-bg-danger-soft);
    border-radius: 4px;
    box-shadow: var(--theme-shadow-soft);
    font-size: 11px;
    line-height: 1.35;
    white-space: normal;
    pointer-events: none;
  }
  .form-field-error::before {
    position: absolute;
    bottom: 100%;
    left: 12px;
    width: 0;
    height: 0;
    border: 6px solid transparent;
    border-bottom-color: var(--theme-bg-danger-soft);
    content: '';
  }
  .form-field.invalid { z-index: 3; }
  .form-field.invalid :global(:is(input, select, textarea, .dropdown-select-trigger, .grouped-select-trigger)) {
    border-color: var(--theme-danger);
    box-shadow: 0 0 0 1px var(--theme-danger);
  }
</style>
