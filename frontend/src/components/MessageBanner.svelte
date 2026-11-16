<script lang="ts">
  import { CircleAlert, CircleCheck, Info, TriangleAlert } from 'lucide-svelte';

  export let message: string;
  export let tone: 'success' | 'error' | 'warning' | 'info' = 'info';
  export let httpStatus: number | undefined = undefined;
  export let messageCode: string | undefined = undefined;
  export let messageType: string | undefined = undefined;

  const fallbackCodes = {
    success: 'S0001',
    error: 'E0045',
    warning: 'W0001',
    info: 'I0001'
  };

  const icons = {
    success: CircleCheck,
    error: CircleAlert,
    warning: TriangleAlert,
    info: Info
  };
  $: Icon = icons[tone];
  $: resolvedMessageCode = messageCode || fallbackCodes[tone];
</script>

<div
  class={`message-banner ${tone}`}
  role={tone === 'error' ? 'alert' : 'status'}
  aria-live="polite"
>
  <span class="message-content">
    <span class="message-codes" aria-label="消息码">
      {#if httpStatus}<code
          class="message-code http"
          aria-label={`HTTP 状态码 ${httpStatus}`}>{httpStatus}</code
        >{/if}
      <code
        class={`message-code kind-${resolvedMessageCode.charAt(0).toLowerCase()}`}
        aria-label={`消息码 ${resolvedMessageCode}`}>{resolvedMessageCode}</code
      >
      {#if messageType}<code
          class="message-type"
          aria-label={`错误类型 ${messageType}`}>{messageType}</code
        >{/if}
    </span>
    <svelte:component
      this={Icon}
      size={13}
      strokeWidth={1.8}
      aria-hidden="true"
    />
    <span class="message-text">{message}</span>
  </span>
</div>
