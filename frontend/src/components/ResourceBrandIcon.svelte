<script lang="ts">
  import {
    FolderGit2,
    Network,
    Package,
    Server,
    Waypoints
  } from 'lucide-svelte';
  import IconValue from './IconValue.svelte';
  import { brandNameFor } from '../lib/resources';
  import BrandIcon from '../lib/BrandIcon.svelte';

  export let resource: {
    kind: string;
    subtype?: string;
    config?: Record<string, unknown>;
  };
  export let fallback = '◇';
  export let size = 18;
  export let brandOnly = false;
  export let monochrome = false;

  $: brand = brandNameFor(resource);
  $: kindIcon = {
    Artifact: Package,
    Repository: FolderGit2,
    Nacos: Network,
    MCPServer: Waypoints
  }[resource.kind];
</script>

<span class:resource-brand-icon-monochrome={monochrome}>
{#if kindIcon && !brandOnly}
  <svelte:component
    this={kindIcon}
    {size}
    strokeWidth={1.8}
    aria-hidden="true"
  />
{:else if resource.kind === 'Host'}
  <Server {size} strokeWidth={1.8} aria-hidden="true" />
{:else if brandOnly && (fallback.startsWith('lucide:') || fallback.startsWith('iconify:') || fallback.startsWith('data:image/'))}
  <IconValue value={fallback} {size} />
{:else if brand}
  <BrandIcon name={brand} {size} />
{:else if fallback.startsWith('lucide:') || fallback.startsWith('iconify:') || fallback.startsWith('data:image/')}
  <IconValue value={fallback} {size} />
{:else}
  <span class="brand-icon-fallback" aria-hidden="true">{fallback}</span>
{/if}
</span>

<style>
  .resource-brand-icon-monochrome {
    display: inline-flex;
    filter: brightness(0) invert(1);
  }
</style>
