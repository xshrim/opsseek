<script lang="ts">
  import type { Resource } from '../../lib/api';
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FormField from '../../components/FormField.svelte';
  import PasswordInput from '../../components/PasswordInput.svelte';
  import TextInput from '../../components/TextInput.svelte';

  export let accessMode: 'direct' | 'agent' = 'direct';
  export let host = '';
  export let port = 27017;
  export let database = '';
  export let authSource = 'admin';
  export let username = '';
  export let password = '';
  export let timeoutSeconds = 10;
  export let mcpServerResourceId = '';
  export let mcpServers: Resource[] = [];
  export let configurationAttempted = false;
  export let editing = false;
  export let credentialConfigured = false;
  export let onConfigurationChange: () => void = () => {};
  $: credentialInvalid = configurationAttempted && Boolean(username.trim()) !== Boolean(password);
  $: mcpServerOptions = mcpServers.map((server) => ({ value: server.id, label: server.name }));
</script>

{#if accessMode === 'agent'}
  <FormField label="关联 MCPServer" required asLabel={false} invalid={configurationAttempted && !mcpServerResourceId}>
    <DropdownSelect bind:value={mcpServerResourceId} options={mcpServerOptions} placeholder="请选择活动的 MCPServer" ariaLabel="关联 MCPServer" invalid={configurationAttempted && !mcpServerResourceId} on:change={onConfigurationChange} />
  </FormField>
  <p class="docker-field-help">MongoDB Agent 通过关联 MCPServer 提供只读诊断工具。</p>
{:else}
  <div class="docker-form-grid">
    <FormField label="数据库主机" required invalid={configurationAttempted && !host.trim()}>
      <TextInput bind:value={host} required invalid={configurationAttempted && !host.trim()} on:input={onConfigurationChange} placeholder="例如 mongodb.example.com" />
    </FormField>
    <FormField label="端口" required>
      <TextInput type="number" min="1" max="65535" bind:value={port} on:input={onConfigurationChange} />
    </FormField>
    <FormField label="数据库名">
      <TextInput bind:value={database} on:input={onConfigurationChange} placeholder="留空使用 admin" />
    </FormField>
    <FormField label="认证数据库">
      <TextInput bind:value={authSource} on:input={onConfigurationChange} placeholder="admin" />
    </FormField>
    <FormField label="用户名" invalid={credentialInvalid}>
      <TextInput bind:value={username} invalid={credentialInvalid} placeholder={editing && credentialConfigured ? '留空以保留已配置凭据' : '可选'} on:input={onConfigurationChange} />
    </FormField>
    <FormField label="密码" invalid={credentialInvalid}>
      <PasswordInput bind:value={password} placeholder={editing && credentialConfigured ? '留空以保留已配置凭据' : '可选'} on:input={onConfigurationChange} ariaLabel="密码" />
    </FormField>
    <FormField label="超时时间（秒）">
      <TextInput type="number" min="1" max="300" bind:value={timeoutSeconds} on:input={onConfigurationChange} />
    </FormField>
  </div>
  <p class="docker-field-help">数据库和凭据可留空；认证数据库默认为 admin。连接凭据会加密保存。</p>
{/if}
