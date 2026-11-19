<script lang="ts">
  import type { Resource } from '../../lib/api';
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FormField from '../../components/FormField.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import {
    resourceCategoryFor,
    resourceCategoryIcon,
    resourceCategoryOptions,
    resourceSubtypeFor,
    resourceSubtypeOptionsFor
  } from './resourceCatalog';

  export let resource: Resource;
  export let name = '';
  export let labels = '';
  export let scopeId = resource.scope_id;
  export let scopeOptions: Array<{ id: string; type: string; name: string }> =
    [];
  export let onSelectScope: (scopeId: string) => void = () => {};
  $: categoryValue = resourceCategoryFor(resource);
  $: subtypeValue = resourceSubtypeFor(resource);
  $: categoryOptions = Object.keys(resourceCategoryOptions)
    .filter((category) => category !== '全部')
    .map((category) => ({
      value: category,
      label: category,
      icon: resourceCategoryIcon(category),
      monochromeIcon: true
    }));
  $: subtypeOptions = resourceSubtypeOptionsFor(resource).map((subtype) => ({
    value: subtype,
    label: subtype,
    icon: resourceCategoryIcon(categoryValue),
    monochromeIcon: true
  }));
  $: scopeDropdownOptions = scopeOptions.map((scope) => ({
    value: scope.id,
    label: `${scope.type === 'platform' ? '平台' : scope.type === 'team' ? '团队' : '项目'} · ${scope.name}`,
    icon:
      scope.type === 'platform'
        ? 'lucide:PanelsTopLeft'
        : scope.type === 'team'
          ? 'lucide:Building'
          : 'lucide:Boxes'
  }));
</script>

<h3 class="editor-section-title">基础配置</h3>
<p class="editor-section-description">
  资源类型和子类型只读，资源名称与资源标签可在此调整。
</p>
<div class="resource-basic-edit-grid">
  <div class="resource-basic-type-row">
    <FormField label="资源类型"
      ><DropdownSelect
        value={categoryValue}
        options={categoryOptions}
        showIcons
        iconTone="resource"
        disabled
        ariaLabel="资源类型"
      /></FormField
    >
    <FormField label="资源子类型"
      ><DropdownSelect
        value={subtypeValue}
        options={subtypeOptions}
        showIcons
        iconTone="resource"
        disabled
        ariaLabel="资源子类型"
      /></FormField
    >
  </div>
  <div class="resource-basic-identity-row">
    <FormField label="资源名称" required
      ><TextInput bind:value={name} required /></FormField
    >
    <FormField label="资源级别" className="resource-basic-level" required
      ><DropdownSelect
        value={scopeId}
        options={scopeDropdownOptions}
        showIcons
        iconTone="scope"
        placeholder="请选择资源级别"
        ariaLabel="资源级别"
        on:change={(event) => onSelectScope(String(event.detail))}
      /></FormField
    >
  </div>
  <FormField label="资源标签" className="resource-basic-labels" asLabel={false}
    ><TextInput
      bind:value={labels}
      tagged
      placeholder="填写 key=value、key:value 或 key，按回车确认"
      autocomplete="off"
    /></FormField
  >
</div>
