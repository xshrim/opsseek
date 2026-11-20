<script lang="ts">
  import FormField from '../../components/FormField.svelte';
  import McpServerSelect from './McpServerSelect.svelte';
  import Switch from '../../components/Switch.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import TlsCredentialFields from './TlsCredentialFields.svelte';
  import type { Resource } from '../../lib/api';
  import {
    dockerHostSupportsTLS,
    dockerHostValid,
    dockerTLSValueValid,
    type DockerAccessMode
  } from './resourceWorkflow';

  export let accessMode: DockerAccessMode = 'direct';
  export let daemonType: 'dockerd' | 'containerd' = 'dockerd';
  export let namespace = 'k8s.io';
  export let host = '';
  export let timeoutSeconds = 10;
  export let caBase64 = '';
  export let certBase64 = '';
  export let keyBase64 = '';
  export let skipTLSVerify = false;
  export let mcpServerResourceId = '';
  export let connectionOverride = false;
  export let mcpServers: Resource[] = [];
  export let allowAddMCPServer = true;
  export let configurationAttempted = false;
  export let credentialLoading = false;
  export let credentialConfigured = false;
  export let onConfigurationChange: () => void = () => {};
  export let onAddMCPServer: () => void = () => {};

  let lastMcpServerResourceId = '';

  $: tlsSupported = dockerHostSupportsTLS(host);
  $: if (mcpServerResourceId !== '__add_mcp_server__')
    lastMcpServerResourceId = mcpServerResourceId;

  function selectMCPServer(event: CustomEvent<string>) {
    const value = event.detail;
    if (value === '__add_mcp_server__') {
      mcpServerResourceId = lastMcpServerResourceId;
      onAddMCPServer();
      return;
    }
    onConfigurationChange();
  }
</script>

<div
  class="docker-daemon-fields"
  class:containerd-daemon={daemonType === 'containerd'}
>
  <FormField label="Daemon 类型" required className="docker-daemon-type-field">
    <select
      bind:value={daemonType}
      on:change={onConfigurationChange}
      aria-label="Daemon 类型"
    >
      <option value="dockerd">dockerd</option>
      <option value="containerd">containerd</option>
    </select>
  </FormField>
  {#if accessMode === 'direct' || connectionOverride}
    <FormField
      label="Daemon 地址"
      required
      invalid={configurationAttempted &&
        (!host.trim() || !dockerHostValid(host))}
      className="docker-daemon-address-field"
    >
      <TextInput
        bind:value={host}
        on:input={onConfigurationChange}
        required
        placeholder={daemonType === 'containerd'
          ? '例如 unix:///run/containerd/containerd.sock 或 tcp://host:2376'
          : '例如 unix:///var/run/docker.sock 或 https://docker.example.com:2376'}
        autocomplete="off"
        ariaLabel="Daemon 地址"
      />
    </FormField>
  {/if}
  {#if daemonType === 'containerd'}
    <FormField
      label="命名空间"
      required
      invalid={configurationAttempted && !namespace.trim()}
      className="docker-daemon-namespace-field"
    >
      <TextInput
        bind:value={namespace}
        on:input={onConfigurationChange}
        required
        placeholder="k8s.io"
        ariaLabel="命名空间"
      />
    </FormField>
  {/if}
  {#if accessMode === 'direct' || connectionOverride}
    <FormField
      label="超时时间（秒）"
      className="docker-daemon-timeout-field"
    >
      <TextInput
        bind:value={timeoutSeconds}
        on:input={onConfigurationChange}
        min="1"
        max="300"
        type="number"
        ariaLabel="超时时间（秒）"
      />
    </FormField>
  {/if}
  {#if accessMode === 'direct' || connectionOverride}
    <p
      id="docker-host-help"
      class="docker-field-help docker-daemon-address-help"
    >
      {#if daemonType === 'containerd'}
        <code>unix:///run/containerd/containerd.sock</code>、
        <code>tcp://host:端口</code>、<code>grpc://host:端口</code>、
        <code>grpcs://host:端口</code>、<code>https://host:端口</code>
      {:else}
        <code>unix:///var/run/docker.sock</code>、
        <code>tcp://host:端口</code>、<code>http://host:端口</code>、
        <code>https://host:端口</code>
      {/if}
    </p>
  {/if}
</div>

{#if accessMode === 'agent'}
  <div class="docker-agent-form">
    <div class="docker-agent-connection-row">
      <McpServerSelect
        bind:value={mcpServerResourceId}
        resources={mcpServers}
        allowAdd={allowAddMCPServer}
        invalid={configurationAttempted && !mcpServerResourceId}
        ariaLabel="关联 MCPServer"
        on:change={selectMCPServer}
      />
      <label class="docker-agent-override-toggle">
        <span>使用自定义 Docker 连接</span>
        <Switch
          bind:checked={connectionOverride}
          ariaLabel="使用自定义 Docker 连接"
          on:change={onConfigurationChange}
        />
      </label>
    </div>
    <p id="docker-agent-help" class="docker-field-help">
      逻辑 Docker 资源仍是权限和审计主体；MCPServer 只提供传输路径。
    </p>
    {#if !mcpServers.length}
      <div class="docker-agent-empty" role="status">
        <strong>没有可用的 MCPServer</strong>
        <span>请先创建并启用一个 MCPServer，再选择 Agent 接入。</span>
      </div>
    {/if}
  </div>
{/if}

{#if accessMode === 'direct' || connectionOverride}
  <form
    id="docker-create-form"
    class="docker-connection-form"
    on:submit|preventDefault
  >
    <TlsCredentialFields
      bind:ca={caBase64}
      bind:cert={certBase64}
      bind:key={keyBase64}
      bind:skipVerify={skipTLSVerify}
      {configurationAttempted}
      {credentialConfigured}
      disabled={!tlsSupported}
      help={tlsSupported
        ? 'CA 证书、客户端证书和私钥支持粘贴 PEM 文本或 Base64 编码，也支持导入 PEM 文件。数据库和工具参数统一使用 Base64；客户端证书和私钥必须成对。'
        : '当前 Docker Host URL 不支持 TLS 配置。请选择 tcp:// 或 https:// 后再配置。'}
      validate={dockerTLSValueValid}
      legend="TLS 客户端配置（可选）"
      onChange={onConfigurationChange}
    />
    {#if credentialLoading}<p class="docker-field-help">
        正在读取已有 TLS 凭据，请稍候…
      </p>{/if}
  </form>
{/if}
