<script lang="ts">
  import ExpandableTable from '../../src/components/ExpandableTable.svelte';
  import ReadonlyTable from '../../src/components/ReadonlyTable.svelte';
  import FormField from '../../src/components/FormField.svelte';
  import TextInput from '../../src/components/TextInput.svelte';
  import ModalDialog from '../../src/components/ModalDialog.svelte';

  const columns = [{ key: 'name', label: '名称' }];
  const rows = [
    { id: 'one', name: '应用一' },
    { id: 'two', name: '应用二' }
  ];
  let dialogOpen = false;
</script>

{#each ['access-member-table', 'resource-expandable-table', 'application-table'] as skin}
  <section aria-label={skin}>
    <ExpandableTable
      {columns}
      {rows}
      className={skin}
      selectable
      expandable
      selectedIds={['one']}
      rowClass={() => 'resource-catalog-row'}
    >
      <svelte:fragment slot="details"
        ><ReadonlyTable
          {columns}
          rows={[{ name: '关联资源' }]}
        /></svelte:fragment
      >
    </ExpandableTable>
  </section>
{/each}
{#each ['resource-row-actions', 'application-table-actions', 'access-row-actions', 'diagnosis-session-actions'] as skin}
  <section
    class:diagnosis-session-item={skin === 'diagnosis-session-actions'}
    style="position: relative; min-height: 32px;"
  >
    <div class={skin} aria-label={skin}>
      <button class="icon-button edit-action" aria-label="编辑">E</button>
      <button class="icon-button danger-action delete-action" aria-label="删除"
        >D</button
      >
    </div>
  </section>
{/each}
<div class="application-dependency-table" aria-label="应用依赖表格">
  <ReadonlyTable {columns} rows={[{ name: '依赖应用' }]} />
</div>
<article class="skill-row" tabindex="0" aria-label="技能行">技能</article>
<div class="drawer-form">
  <FormField label="错误字段" invalid error="请输入名称">
    <TextInput ariaLabel="错误字段" />
  </FormField>
</div>
<button onclick={() => (dialogOpen = true)}>打开确认窗口</button>
{#if dialogOpen}
  <ModalDialog
    title="确认操作"
    size="small"
    dialogClass="confirm-dialog"
    onCancel={() => (dialogOpen = false)}
  >
    <p>确认内容</p>
  </ModalDialog>
{/if}
