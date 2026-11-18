<script lang="ts">
  import FormActions from '../../components/FormActions.svelte';
  import ModalDialog from '../../components/ModalDialog.svelte';
  import { Plus, RefreshCw, Trash2, Users } from 'lucide-svelte';
  import {
    api,
    type ExpertGroup,
    type ExpertGroupBinding,
    type ExpertGroupMember,
    type PersonaCatalogItem
  } from '../../lib/api';
  import { errorMessage as formatErrorMessage } from '../../lib/errorMessage';
  import FormField from '../../components/FormField.svelte';
  import TextArea from '../../components/TextArea.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import DropdownSelect from '../../components/DropdownSelect.svelte';

  export let scopeId = '';
  export let profiles: PersonaCatalogItem[] = [];
  export let onNotice: (message: string) => void;
  export let onError: (message: string) => void;

  let groups: ExpertGroup[] = [];
  let bindings: ExpertGroupBinding[] = [];
  let selectedGroupId = '';
  let loading = false;
  let busy = false;
  let createOpen = false;
  let createBusy = false;
  let createIdentifier = '';
  let createName = '';
  let createDescription = '';
  let memberRows: ExpertGroupMember[] = [];
  let bindingSceneTag = 'diagnosis';
  let bindingGroupId = '';

  const sceneTagOptions = [
    { value: 'general', label: '通用 general' },
    { value: 'diagnosis', label: '诊断 diagnosis' },
    { value: 'inspection', label: '巡检 inspection' },
    { value: 'workflow', label: '工作流 workflow' }
  ];

  $: selectedGroup = groups.find((group) => group.id === selectedGroupId) ?? null;
  $: workerProfiles = profiles.filter(
    (profile) =>
      profile.status === 'active' &&
      String(profile.config?.role ?? 'worker') === 'worker'
  );
  $: managerProfiles = profiles.filter(
    (profile) =>
      profile.status === 'active' &&
      String(profile.config?.role ?? '') === 'manager'
  );

  function describeError(error: unknown, fallback: string) {
    return formatErrorMessage(error, fallback, {
      forbidden: '当前账号没有执行此操作的权限。',
      unauthorized: '会话已过期，请重新登录。'
    });
  }

  function personaLabel(personaId: string): string {
    const profile = profiles.find((item) => item.id === personaId);
    return profile ? `${profile.name}（${profile.identifier}）` : personaId;
  }

  async function load() {
    if (!scopeId) return;
    loading = true;
    try {
      const [groupResult, bindingResult] = await Promise.all([
        api.expertGroups(scopeId),
        api.expertGroupBindings(scopeId)
      ]);
      groups = groupResult.items;
      bindings = bindingResult.items;
      if (selectedGroupId && !groups.some((g) => g.id === selectedGroupId)) {
        selectedGroupId = '';
        memberRows = [];
      }
      if (selectedGroupId) await loadMembers(selectedGroupId);
    } catch (error) {
      onError(describeError(error, '专家组加载失败'));
    } finally {
      loading = false;
    }
  }

  async function loadMembers(groupId: string) {
    try {
      const result = await api.expertGroupMembers(groupId);
      memberRows = result.items;
    } catch (error) {
      onError(describeError(error, '专家组成员加载失败'));
    }
  }

  function selectGroup(group: ExpertGroup) {
    selectedGroupId = group.id;
    void loadMembers(group.id);
  }

  function startCreate() {
    createIdentifier = '';
    createName = '';
    createDescription = '';
    createOpen = true;
  }

  async function createGroup() {
    if (!scopeId || !createIdentifier.trim() || !createName.trim()) return;
    createBusy = true;
    try {
      const item = await api.createExpertGroup({
        scope_id: scopeId,
        identifier: createIdentifier.trim(),
        name: createName.trim(),
        description: createDescription.trim()
      });
      createOpen = false;
      await load();
      selectedGroupId = item.id;
      memberRows = [];
      onNotice(`专家组“${item.name}”已创建，请添加成员`);
    } catch (error) {
      onError(describeError(error, '创建专家组失败'));
    } finally {
      createBusy = false;
    }
  }

  function addMemberRow() {
    if (memberRows.length >= 20) return;
    const hasLead = memberRows.some((row) => row.seat === 'lead');
    memberRows = [
      ...memberRows,
      {
        persona_id: '',
        seat: hasLead ? 'assistant' : 'lead',
        scene_tags: [],
        priority: 100
      }
    ];
  }

  function removeMemberRow(index: number) {
    memberRows = memberRows.filter((_, i) => i !== index);
  }

  async function saveMembers() {
    if (!selectedGroupId) return;
    busy = true;
    try {
      await api.replaceExpertGroupMembers(selectedGroupId, memberRows);
      await loadMembers(selectedGroupId);
      onNotice('专家组成员已保存');
    } catch (error) {
      onError(describeError(error, '保存成员失败：席位需满足 lead=职能专家、assistant=资源专家且仅一个 lead'));
    } finally {
      busy = false;
    }
  }

  async function saveBinding() {
    if (!scopeId || !bindingGroupId) return;
    busy = true;
    try {
      await api.setExpertGroupBinding(scopeId, {
        scene_tag: bindingSceneTag,
        group_id: bindingGroupId
      });
      await load();
      onNotice('专家组绑定已保存');
    } catch (error) {
      onError(describeError(error, '保存绑定失败'));
    } finally {
      busy = false;
    }
  }

  async function removeBinding(binding: ExpertGroupBinding) {
    busy = true;
    try {
      await api.removeExpertGroupBinding(
        binding.scope_id,
        binding.scene_tag,
        binding.group_id,
        binding.application_id
      );
      await load();
      onNotice('已解除绑定');
    } catch (error) {
      onError(describeError(error, '解除绑定失败'));
    } finally {
      busy = false;
    }
  }

  async function toggleStatus(group: ExpertGroup) {
    busy = true;
    try {
      const next = group.status === 'active' ? 'disabled' : 'active';
      await api.updateExpertGroup(group.id, { status: next });
      await load();
      onNotice(next === 'active' ? '专家组已启用' : '专家组已停用（等价于无绑定）');
    } catch (error) {
      onError(describeError(error, '更新专家组失败'));
    } finally {
      busy = false;
    }
  }
</script>

<section class="panel expert-groups-panel">
  <header class="expert-groups-header">
    <div>
      <strong>专家组</strong>
      <p>
        按系统配备的专家团队：lead（职能专家）+ assistant（资源专家）。绑定后诊断/巡检按
        应用级 → 场景精确 → general 兜底解析。
      </p>
    </div>
    <div class="expert-groups-actions">
      <button class="secondary" type="button" on:click={load}
        ><RefreshCw size={14} />刷新</button
      ><button class="primary" type="button" on:click={startCreate}
        ><Plus size={14} />新增专家组</button
      >
    </div>
  </header>

  {#if groups.length === 0 && !loading}
    <div class="expert-groups-empty">
      <Users size={24} />
      <span>尚无专家组。创建一个组并添加成员席位。</span>
    </div>
  {:else}
    <div class="expert-groups-list">
      {#each groups as group}
        <div
          class:selected={group.id === selectedGroupId}
          class="expert-group-item"
          role="button"
          tabindex="0"
          on:click={() => selectGroup(group)}
          on:keydown={(event) =>
            (event.key === 'Enter' || event.key === ' ') && selectGroup(group)}
        >
          <strong>{group.name}</strong>
          <small>{group.identifier} · {group.members?.length ?? 0} 成员</small>
          <span class:active={group.status === 'active'}
            >{group.status === 'active' ? '启用' : '停用'}</span
          >
          <button
            class="secondary"
            type="button"
            on:click|stopPropagation={() => toggleStatus(group)}
            >{group.status === 'active' ? '停用' : '启用'}</button
          >
        </div>
      {/each}
    </div>
  {/if}

  {#if selectedGroup}
    <div class="expert-group-detail">
      <h4>成员席位 · {selectedGroup.name}</h4>
      <p class="muted-copy">
        lead 仅允许职能专家（Manager），assistant 仅允许资源专家（Worker）；
        lead 自带的委派列表在专家组执行时会被忽略。
      </p>
      <div class="member-rows">
        {#each memberRows as row, index}
          <div class="member-row">
            <DropdownSelect
              value={row.persona_id}
              options={(row.seat === 'lead' ? managerProfiles : workerProfiles).map(
                (profile) => ({
                  value: profile.id,
                  label: `${profile.name}（${profile.identifier}）`
                })
              )}
              placeholder="选择专家"
              ariaLabel="成员专家"
              onChange={(value: string) => {
                memberRows[index] = { ...row, persona_id: value };
                memberRows = memberRows;
              }}
            />
            <DropdownSelect
              value={row.seat}
              options={[
                { value: 'lead', label: '主诊 lead' },
                { value: 'assistant', label: '辅诊 assistant' }
              ]}
              ariaLabel="席位"
              onChange={(value: string) => {
                memberRows[index] = {
                  ...row,
                  seat: value as ExpertGroupMember['seat']
                };
                memberRows = memberRows;
              }}
            />
            <DropdownSelect
              value={row.scene_tags}
              options={[
                { value: 'diagnosis', label: '诊断' },
                { value: 'inspection', label: '巡检' },
                { value: 'workflow', label: '工作流' }
              ]}
              multiple
              placeholder="全部场景"
              ariaLabel="生效场景"
              onChange={(value: string[]) => {
                memberRows[index] = { ...row, scene_tags: value };
                memberRows = memberRows;
              }}
            />
            <button
              class="icon-button"
              type="button"
              aria-label="移除成员"
              on:click={() => removeMemberRow(index)}><Trash2 size={14} /></button
            >
          </div>
        {/each}
      </div>
      <FormActions>
        <button class="secondary" type="button" on:click={addMemberRow}
          ><Plus size={14} />添加成员</button
        ><button
          class="primary"
          type="button"
          on:click={saveMembers}
          disabled={busy || memberRows.length === 0}>保存成员</button
        >
      </FormActions>
    </div>
  {/if}

  <div class="expert-group-detail">
    <h4>场景默认绑定（当前范围）</h4>
    <div class="binding-editor">
      <DropdownSelect
        bind:value={bindingSceneTag}
        options={sceneTagOptions}
        ariaLabel="场景标签"
      />
      <DropdownSelect
        bind:value={bindingGroupId}
        options={groups.map((group) => ({
          value: group.id,
          label: group.name
        }))}
        placeholder="选择专家组"
        ariaLabel="专家组"
      />
      <button class="primary" type="button" on:click={saveBinding} disabled={busy}
        >保存绑定</button
      >
    </div>
    <ul class="binding-list">
      {#each bindings as binding}
        <li>
          <span>{binding.scene_tag}</span>
          <span>{groups.find((g) => g.id === binding.group_id)?.name ?? binding.group_id}</span>
          {#if binding.application_id}<small>应用级</small>{/if}
          <button
            class="icon-button"
            type="button"
            aria-label="解除绑定"
            on:click={() => removeBinding(binding)}><Trash2 size={14} /></button
          >
        </li>
      {:else}
        <li class="muted-copy">当前范围暂无绑定。</li>
      {/each}
    </ul>
  </div>
</section>

{#if createOpen}
  <ModalDialog
    title="新增专家组"
    description="创建可绑定到应用或场景默认的专家团队。"
    confirmLabel="创建"
    confirmForm="create-expert-group-form"
    busy={createBusy}
    confirmDisabled={!scopeId || !createIdentifier.trim() || !createName.trim()}
    onCancel={() => (createOpen = false)}
  >
    <form
      id="create-expert-group-form"
      on:submit|preventDefault={createGroup}
    >
      <FormField label="唯一标识" required
        ><TextInput
          bind:value={createIdentifier}
          required
          placeholder="如 dba-team"
          ariaLabel="专家组唯一标识"
      /></FormField>
      <FormField label="名称" required
        ><TextInput
          bind:value={createName}
          required
          placeholder="如 数据库专家组"
          ariaLabel="专家组名称"
      /></FormField>
      <FormField label="说明"><TextArea bind:value={createDescription} rows={3} /></FormField>
    </form>
  </ModalDialog>
{/if}
