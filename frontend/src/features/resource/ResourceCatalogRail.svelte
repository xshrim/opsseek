<script lang="ts">
  import { onMount } from 'svelte';
  import type { Resource } from '../../lib/api';
  import { Activity, Boxes, Database, Filter, HardDrive, Network, Package, Server, Waypoints, X } from 'lucide-svelte';
  import ResourceBrandIcon from '../../components/ResourceBrandIcon.svelte';
  import SearchInput from '../../components/SearchInput.svelte';
  import {
    resourceCategoryFor,
    resourceCategoryIcon,
    resourceCategoryOptions,
    resourceCatalogDomainOrder,
    resourceSubtypeFor,
    resourceCatalogTagOptions,
    resourceCatalogTagsFor,
    resourceCatalogTagClass,
    resourceDomainFor
  } from './resourceCatalog';

  export let resources: Resource[] = [];
  export let category = '全部';
  export let subtype = '全部';
  export let onSelect: (category: string, subtype?: string) => void = () => {};
  export let selectedCatalogTags: string[] = [];
  export let catalogQuery = '';
  export let onCatalogQuery: (value: string) => void = () => {};
  export let onToggleCatalogTag: (tag: string) => void = () => {};

  let expandedCategory = '';
  let expandedDomain = resourceCatalogDomainOrder[0] ?? '';
  let filterOpen = false;
  let filterRoot: HTMLDivElement;

  const domainIcons = {
    运行时: Server,
    大模型: Waypoints,
    工程库: Package,
    数据库: Database,
    存储库: HardDrive,
    中间件: Boxes,
    微服务: Network,
    可观测: Activity
  };

  function closeFilterOnOutside(event: PointerEvent) {
    if (filterOpen && filterRoot && !filterRoot.contains(event.target as Node))
      filterOpen = false;
  }

  onMount(() => {
    window.addEventListener('pointerdown', closeFilterOnOutside);
    return () =>
      window.removeEventListener('pointerdown', closeFilterOnOutside);
  });

  $: filteredCategories = Object.entries(resourceCategoryOptions).filter(
    ([categoryName]) => {
      if (categoryName === '全部') return false;
      const tags = resourceCatalogTagsFor(categoryName);
      const query = catalogQuery.trim().toLowerCase();
      const hasQuery = Boolean(query);
      const hasTags = selectedCatalogTags.length > 0;
      const matchesQuery = [categoryName, resourceDomainFor(categoryName), ...tags]
        .join(' ')
        .toLowerCase()
        .includes(query);
      const matchesTags = selectedCatalogTags.some((tag) => tags.includes(tag));
      if (!hasQuery && !hasTags) return true;
      return (hasQuery && matchesQuery) || (hasTags && matchesTags);
    }
  );
  $: groupedCategories = resourceCatalogDomainOrder
    .map((domain) => ({
      domain,
      categories: filteredCategories.filter(
        ([categoryName]) => resourceDomainFor(categoryName) === domain
      )
    }))
    .filter((group) => group.categories.length > 0);

  function select(categoryName: string, subtypeName = '全部') {
    expandedCategory =
      categoryName === '全部'
        ? ''
        : subtypeName === '全部' && expandedCategory === categoryName
          ? ''
          : categoryName;
    onSelect(categoryName, subtypeName);
  }
</script>

<section class="panel resource-list-panel">
  <div class="resource-catalog-rail">
    <button
      class:active={category === '全部'}
      class="catalog-root"
      type="button"
      on:click={() => select('全部')}
    >
      <span class="catalog-label">全部资源</span><span>{resources.length}</span>
    </button>
    <div class="resource-catalog-filter" bind:this={filterRoot}>
      <SearchInput
        className="resource-catalog-search-wrap"
        value={catalogQuery}
        on:value={(event) => onCatalogQuery(event.detail)}
        width="100%"
        height="28px"
        placeholder="搜索资源类型或标签"
        ariaLabel="搜索资源类型或标签"
      >
        <svelte:fragment slot="prefix"
          >{#each selectedCatalogTags as tag}<button
              class={`resource-catalog-selected-tag resource-catalog-tag-${resourceCatalogTagClass(tag)}`}
              type="button"
              data-tooltip={`移除筛选 ${tag}`}
              aria-label={`移除筛选 ${tag}`}
              on:click={() => onToggleCatalogTag(tag)}
              >{tag}<X size={11} aria-hidden="true" /></button
            >{/each}</svelte:fragment
        >
        <svelte:fragment slot="suffix"
          ><button
            class:active={filterOpen || selectedCatalogTags.length > 0}
            class="icon-button resource-catalog-filter-trigger"
            type="button"
            on:click={() => (filterOpen = !filterOpen)}
            data-tooltip="按 Catalog 标签筛选"
            aria-label="按 Catalog 标签筛选"
            aria-expanded={filterOpen}
            ><Filter size={14} aria-hidden="true" /></button
          ></svelte:fragment
        >
      </SearchInput>
      {#if filterOpen}
        <div
          class="resource-catalog-filter-popover"
          role="group"
          aria-label="Catalog 标签"
        >
          {#each resourceCatalogTagOptions as tag}
            <button
              class:selected={selectedCatalogTags.includes(tag)}
              class="resource-catalog-filter-option"
              type="button"
              on:click={() => onToggleCatalogTag(tag)}
              ><span
                class={`resource-catalog-tag resource-catalog-tag-${resourceCatalogTagClass(tag)}`}
                >{tag}</span
              ></button
            >
          {/each}
        </div>
      {/if}
    </div>
    {#each groupedCategories as group}
      <section class="catalog-domain-group" aria-label={group.domain}>
        <button
          class="catalog-domain-heading"
          type="button"
          aria-expanded={expandedDomain === group.domain}
          on:click={() =>
            (expandedDomain = expandedDomain === group.domain ? '' : group.domain)}
        >
          <svelte:component this={domainIcons[group.domain as keyof typeof domainIcons]} size={15} aria-hidden="true" />
          <strong>{group.domain}</strong>
        </button>
        {#if expandedDomain === group.domain}
        {#each group.categories as [categoryName, subtypes]}
        <div class="catalog-category">
        <button
          class:active={category === categoryName && subtype === '全部'}
          class="catalog-category-button"
          type="button"
          on:click={() => select(categoryName)}
        >
          <span class="catalog-name"
            ><span class="catalog-icon"
              ><ResourceBrandIcon
                resource={{ kind: categoryName }}
                fallback={resourceCategoryIcon(categoryName)}
                size={20}
              /></span
            >{categoryName}{#each resourceCatalogTagsFor(categoryName) as tag}<em
                class={`resource-catalog-tag resource-catalog-tag-${resourceCatalogTagClass(tag)}`}
                >{tag}</em
              >{/each}</span
          >
          <span
            >{resources.filter(
              (item) => resourceCategoryFor(item) === categoryName
            ).length}</span
          >
        </button>
        {#if expandedCategory === categoryName}
          <div class="catalog-subtypes">
            {#each subtypes as subtypeName}
              <button
                class:active={category === categoryName &&
                  subtype === subtypeName}
                type="button"
                on:click={() => select(categoryName, subtypeName)}
              >
                <span class="catalog-name"
                  ><span class="catalog-icon subtype-icon"
                    ><ResourceBrandIcon
                      resource={{ kind: categoryName, subtype: subtypeName }}
                      fallback={resourceCategoryIcon(subtypeName)}
                      size={15}
                    /></span
                  >{subtypeName}</span
                >
                <span
                  >{resources.filter(
                    (item) =>
                      resourceCategoryFor(item) === categoryName &&
                      resourceSubtypeFor(item) === subtypeName
                  ).length}</span
                >
              </button>
            {/each}
          </div>
        {/if}
        </div>
        {/each}
        {/if}
      </section>
    {/each}
  </div>
</section>
