<script lang="ts">
  import { CircleAlert, CircleCheck, Info, TriangleAlert } from 'lucide-svelte';
  import { onDestroy, onMount, tick } from 'svelte';

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

  let viewport: HTMLDivElement;
  let content: HTMLDivElement;
  let scrolling = false;
  let resizeObserver: ResizeObserver | undefined;

  function updateOverflow() {
    if (viewport && content) {
      scrolling = content.scrollHeight > viewport.clientHeight + 1;
    }
  }

  $: {
    message;
    httpStatus;
    messageCode;
    messageType;
    tick().then(updateOverflow);
  }

  onMount(() => {
    resizeObserver = new ResizeObserver(updateOverflow);
    if (viewport) resizeObserver.observe(viewport);
    if (content) resizeObserver.observe(content);
    updateOverflow();
  });

  onDestroy(() => resizeObserver?.disconnect());
</script>

<div
  class={`message-banner ${tone}`}
  role={tone === 'error' ? 'alert' : 'status'}
  aria-live="polite"
>
  <div class="message-viewport" bind:this={viewport}>
    <div class="message-track" class:scrolling>
      <div class="message-content" bind:this={content}>
        <span class="message-codes" aria-label="消息码">
          {#if httpStatus}<code
              class="message-code http"
              aria-label={`HTTP 状态码 ${httpStatus}`}>{httpStatus}</code
            >{/if}
          <code
            class={`message-code kind-${resolvedMessageCode.charAt(0).toLowerCase()}`}
            aria-label={`消息码 ${resolvedMessageCode}`}
            >{resolvedMessageCode}</code
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
      </div>
      {#if scrolling}
        <div class="message-content" aria-hidden="true">
          <span class="message-codes">
            {#if httpStatus}<code class="message-code http">{httpStatus}</code
              >{/if}
            <code
              class={`message-code kind-${resolvedMessageCode.charAt(0).toLowerCase()}`}
              >{resolvedMessageCode}</code
            >
            {#if messageType}<code class="message-type">{messageType}</code
              >{/if}
          </span>
          <svelte:component
            this={Icon}
            size={13}
            strokeWidth={1.8}
            aria-hidden="true"
          />
          <span class="message-text">{message}</span>
        </div>
      {/if}
    </div>
  </div>
</div>
