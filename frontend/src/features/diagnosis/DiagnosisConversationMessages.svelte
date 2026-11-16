<script lang="ts">
  import { afterUpdate, onDestroy } from 'svelte';
  import {
    Bot,
    Check,
    Copy,
    Pencil,
    SearchCheck,
    Stethoscope,
    Wrench,
    X
  } from 'lucide-svelte';
  import type { DiagnosisMessage, DiagnosisSnapshot } from '../../lib/api';
  import { diagnosisDurationMilliseconds } from './diagnosisUtils';
  import { diagnosisInterruptionReason } from './diagnosisSession';
  import {
    diagnosisDayKey,
    diagnosisDayLabel,
    diagnosisHasOlderConversation
  } from './diagnosisConversation';

  type DiagnosisLiveTimelineItem = {
    id: number;
    kind: 'analysis' | 'action' | 'investigation';
    text?: string;
    tool?: string;
    label?: string;
    detail?: string;
    status?: string;
    duration?: string;
    elapsed?: string;
    iteration?: number;
    input?: string;
    output?: string;
    actions?: DiagnosisLiveTimelineItem[];
    createdAt?: string;
    updatedAt?: string;
  };

  export let diagnosisSnapshot: DiagnosisSnapshot | null = null;
  export let diagnosisMessageListElement: HTMLDivElement | null = null;
  export let busy = false;
  export let diagnosisGenerating = false;
  export let diagnosisAnswerCompleted = false;
  export let diagnosisStreamingText = '';
  export let diagnosisInterruptedReason = '';
  export let diagnosisEditingMessageId = '';
  export let diagnosisEditDraft = '';
  export let diagnosisProcessExpanded: Record<string, boolean> = {};
  export let diagnosisActionExpanded: Record<string, boolean> = {};
  export let diagnosisLiveProcessExpanded = false;
  export let diagnosisStreamingStartedAt = 0;
  export let formatDate: (value: string) => string;
  export let renderMarkdown: (
    text: string,
    options?: { renderMermaid?: boolean }
  ) => string;
  export let diagnosisLiveTimeline: (
    snapshot: DiagnosisSnapshot | null
  ) => DiagnosisLiveTimelineItem[];
  export let diagnosisHistoryTimeline: (
    snapshot: DiagnosisSnapshot | null,
    assistantIndex: number
  ) => DiagnosisLiveTimelineItem[] = (snapshot) =>
    diagnosisLiveTimeline(snapshot);
  export let diagnosisStatusLabel: (status: string) => string;
  export let diagnosisHasRunningActions: (
    snapshot: DiagnosisSnapshot | null
  ) => boolean;
  export let diagnosisHasPersistedNewAnswer: (
    snapshot: DiagnosisSnapshot | null
  ) => boolean;
  export let diagnosisShouldShowEmptyAnswer: () => boolean = () => false;
  export let diagnosisActionLabel: (item: DiagnosisLiveTimelineItem) => string;
  export let isLastDiagnosisUser: (index: number) => boolean;
  export let beginDiagnosisEdit: (message: DiagnosisMessage) => void;
  export let saveDiagnosisEdit: () => void | Promise<void>;
  export let copyDiagnosisMessage: (
    message: DiagnosisMessage,
    processExpanded: boolean,
    onCopied: () => void
  ) => void;
  export let copyStreamingAnswer: (onCopied: () => void) => void;
  export let copyDiagnosisAction: (content: string) => void;

  let liveTimeline: DiagnosisLiveTimelineItem[] = [];
  let editingMessageSize: { width: number; height: number } | null = null;
  type DisplayMessage = {
    message: DiagnosisMessage;
    streaming: boolean;
    assistantIndex: number;
  };
  let displayMessages: DisplayMessage[] = [];
  let interruptionReason = '';
  let collapsedDays: Record<string, boolean> = {};
  let collapsedSessionID = '';
  let copiedMessageKey = '';
  let copiedMessageTimer: ReturnType<typeof setTimeout> | undefined;
  let mermaidLoader: Promise<(typeof import('mermaid'))['default']> | null =
    null;
  let mermaidSequence = 0;
  // Keep the derived event view stable within one render pass. The reducer is
  // pure, but rebuilding the same timeline in every branch made streaming
  // updates needlessly expensive and obscured which view was being rendered.
  $: liveTimeline = diagnosisLiveTimeline(diagnosisSnapshot);
  $: interruptionReason = diagnosisInterruptionReason(
    diagnosisSnapshot?.session.status,
    diagnosisSnapshot?.session.error_message,
    diagnosisInterruptedReason
  );
  $: if (diagnosisSnapshot?.session.id !== collapsedSessionID) {
    collapsedSessionID = diagnosisSnapshot?.session.id ?? '';
    collapsedDays = {};
  }
  $: {
    const messages: DisplayMessage[] = diagnosisSnapshot
      ? diagnosisSnapshot.messages.map((message, index) => ({
          message,
          streaming: false,
          assistantIndex: diagnosisSnapshot.messages
            .slice(0, index)
            .filter((item) => item.role === 'assistant').length
        }))
      : [];
    if (
      diagnosisSnapshot &&
      !diagnosisHasPersistedNewAnswer(diagnosisSnapshot) &&
      (diagnosisStreamingText.trim() ||
        diagnosisAnswerCompleted ||
        diagnosisGenerating ||
        diagnosisHasRunningActions(diagnosisSnapshot))
    ) {
      messages.push({
        message: {
          id: '__diagnosis_streaming__',
          session_id: diagnosisSnapshot.session.id,
          role: 'assistant',
          content: diagnosisStreamingText,
          created_at: diagnosisStreamingStartedAt
            ? new Date(diagnosisStreamingStartedAt).toISOString()
            : new Date().toISOString()
        },
        streaming: true,
        assistantIndex: diagnosisSnapshot.messages.filter(
          (item) => item.role === 'assistant'
        ).length
      });
    }
    displayMessages = messages;
  }

  $: if (!diagnosisEditingMessageId) editingMessageSize = null;

  function beginMessageEdit(message: DiagnosisMessage, event: MouseEvent) {
    const article = (event.currentTarget as HTMLElement).closest<HTMLElement>(
      '.diagnosis-message-f'
    );
    const bubble = article?.querySelector<HTMLElement>('.diagnosis-bubble-f');
    const content = article?.querySelector<HTMLElement>(
      '.diagnosis-message-content'
    );
    if (bubble && content) {
      const bubbleRect = bubble.getBoundingClientRect();
      const contentWidth = content.getBoundingClientRect().width;
      const extraEditingSpace = Math.max(96, Math.round(contentWidth * 0.2));
      editingMessageSize = {
        width: Math.min(
          contentWidth,
          Math.max(contentWidth * 0.25, bubbleRect.width + extraEditingSpace)
        ),
        height: Math.max(34, bubbleRect.height)
      };
    }
    beginDiagnosisEdit(message);
  }

  function cancelMessageEdit() {
    editingMessageSize = null;
    diagnosisEditingMessageId = '';
  }

  async function saveMessageEdit() {
    await saveDiagnosisEdit();
  }

  const processActionCount = (timeline: DiagnosisLiveTimelineItem[]) =>
    timeline.reduce(
      (count, item) =>
        count +
        (item.kind === 'action' ? (item.actions?.length ?? 1) : 0) +
        (item.kind === 'investigation' ? 1 : 0),
      0
    );

  const processDuration = (timeline: DiagnosisLiveTimelineItem[]) => {
    const first = timeline.find((item) => item.createdAt)?.createdAt;
    const last = [...timeline]
      .reverse()
      .find((item) => item.updatedAt)?.updatedAt;
    if (!first || !last) return '—';
    const milliseconds = Math.max(
      0,
      new Date(last).getTime() - new Date(first).getTime()
    );
    return diagnosisDurationMilliseconds(milliseconds);
  };

  const toggleExpanded = (key: string) => {
    diagnosisActionExpanded = {
      ...diagnosisActionExpanded,
      [key]: !diagnosisActionExpanded[key]
    };
  };

  function toggleDay(dayKey: string) {
    collapsedDays = { ...collapsedDays, [dayKey]: !collapsedDays[dayKey] };
  }

  function markMessageCopied(key: string) {
    copiedMessageKey = key;
    if (copiedMessageTimer) clearTimeout(copiedMessageTimer);
    copiedMessageTimer = setTimeout(() => {
      if (copiedMessageKey === key) copiedMessageKey = '';
    }, 1500);
  }

  function loadMermaid() {
    mermaidLoader ??= import('mermaid').then(({ default: mermaid }) => {
      mermaid.initialize({
        startOnLoad: false,
        securityLevel: 'strict',
        theme: document.documentElement.dataset.theme === 'dark' ? 'dark' : 'default',
        flowchart: { htmlLabels: false }
      });
      return mermaid;
    });
    return mermaidLoader;
  }

  async function renderMermaidDiagrams() {
    const root = diagnosisMessageListElement;
    if (!root) return;
    const holders = [...root.querySelectorAll<HTMLElement>('[data-mermaid-source]')].filter(
      (holder) => !holder.dataset.mermaidRendered
    );
    if (!holders.length) return;
    for (const holder of holders) holder.dataset.mermaidRendered = 'pending';
    let mermaid: (typeof import('mermaid'))['default'];
    try {
      mermaid = await loadMermaid();
    } catch {
      return;
    }
    for (const holder of holders) {
      try {
        const source = decodeURIComponent(holder.dataset.mermaidSource ?? '');
        const { svg } = await mermaid.render(
          `diagnosis-mermaid-${++mermaidSequence}`,
          source
        );
        if (!holder.isConnected) continue;
        holder.innerHTML = svg;
        holder.dataset.mermaidRendered = 'true';
      } catch {
        // Keep the escaped source-code fallback already in the holder.
        holder.dataset.mermaidRendered = 'failed';
      }
    }
  }

  afterUpdate(() => {
    void renderMermaidDiagrams();
  });

  onDestroy(() => {
    if (copiedMessageTimer) clearTimeout(copiedMessageTimer);
  });
</script>

<div class="diagnosis-message-list-f" bind:this={diagnosisMessageListElement}>
  {#if !diagnosisSnapshot && !displayMessages.length}
    <div class="diagnosis-welcome">
      <span class="diagnosis-welcome-icon"><Stethoscope size={23} /></span>
      <h2>从一个问题开始</h2>
      <p>
        可直接提问；如需查询资源状态、内容或性能，请在右侧选择已授权的上下文资源。
        Engine 会按需调用受控只读工具。
      </p>
    </div>
  {/if}
  {#if displayMessages.length}
    {@const showDaySeparators = diagnosisHasOlderConversation(
      displayMessages[0].message.created_at
    )}
    {#each displayMessages as item, index}
      {@const message = item.message}
      {@const isStreaming = item.streaming}
      {@const dayKey = diagnosisDayKey(message.created_at)}
      {@const previousMessage = displayMessages[index - 1]?.message}
      {@const showDaySeparator =
        !previousMessage ||
        diagnosisDayKey(previousMessage.created_at) !== dayKey}
      {@const processTimeline =
        message.role === 'assistant'
          ? isStreaming
            ? liveTimeline
            : diagnosisHistoryTimeline(diagnosisSnapshot, item.assistantIndex)
          : []}
      {#if showDaySeparators && showDaySeparator}
        <button
          type="button"
          class="diagnosis-day-separator"
          aria-expanded={!collapsedDays[dayKey]}
          aria-label={`${collapsedDays[dayKey] ? '展开' : '折叠'} ${diagnosisDayLabel(message.created_at)} 对话`}
          on:click={() => toggleDay(dayKey)}
        >
          <span>{diagnosisDayLabel(message.created_at)}</span>
        </button>
      {/if}
      {#if !collapsedDays[dayKey]}<article
          class="diagnosis-message-f {message.role} {isStreaming
            ? 'diagnosis-streaming-message'
            : ''}"
          data-diagnosis-message-id={isStreaming ? undefined : message.id}
          data-diagnosis-message-content={!isStreaming &&
          message.role === 'user'
            ? message.content
            : undefined}
        >
          <span class="diagnosis-message-avatar"
            >{#if message.role === 'assistant'}<Bot
                size={15}
                strokeWidth={1.8}
                aria-hidden="true"
              />{:else}你{/if}</span
          >
          <div class="diagnosis-message-content">
            <div class="diagnosis-message-meta">
              <small>{formatDate(message.created_at)}</small>
            </div>
            {#if diagnosisEditingMessageId === message.id}
              <div
                class="diagnosis-edit-box"
                style={`width:${editingMessageSize ? `${editingMessageSize.width}px` : '25%'};height:${editingMessageSize?.height ?? 34}px`}
              >
                <textarea
                  bind:value={diagnosisEditDraft}
                  aria-label="编辑诊断问题"
                  rows="1"
                ></textarea>
                <div class="diagnosis-edit-actions">
                  <button
                    class="secondary"
                    aria-label="取消编辑"
                    data-tooltip="取消编辑"
                    on:click={cancelMessageEdit}><X size={13} /></button
                  ><button
                    class="primary"
                    aria-label="重新发送"
                    data-tooltip="重新发送"
                    on:click={() => void saveMessageEdit()}
                    disabled={busy}><Check size={13} /></button
                  >
                </div>
              </div>
              {#if message.role === 'user'}
                <div
                  class="diagnosis-user-actions diagnosis-user-actions-placeholder"
                  aria-hidden="true"
                ></div>
              {/if}
            {:else}
              {#if message.role === 'assistant' && processTimeline.length}
                <svelte:element
                  this={isStreaming && !diagnosisAnswerCompleted
                    ? 'div'
                    : 'details'}
                  class="diagnosis-process"
                  open={isStreaming
                    ? diagnosisAnswerCompleted && diagnosisLiveProcessExpanded
                    : diagnosisProcessExpanded[message.id]}
                  on:toggle={(event: Event) => {
                    const details = event.currentTarget as HTMLDetailsElement;
                    if (isStreaming)
                      diagnosisLiveProcessExpanded = details.open;
                    else
                      diagnosisProcessExpanded = {
                        ...diagnosisProcessExpanded,
                        [message.id]: details.open
                      };
                  }}
                >
                  {#if !isStreaming || diagnosisAnswerCompleted}<summary
                      class="diagnosis-process-summary"
                    >
                      <span class="diagnosis-process-title">执行过程</span>
                      <span class="diagnosis-process-meta"
                        >总耗时 {processDuration(processTimeline)} · {#if processActionCount(processTimeline)}{processActionCount(
                            processTimeline
                          )} 个动作 ·
                        {/if}{isStreaming
                          ? diagnosisStatusLabel(
                              diagnosisSnapshot?.session.status ?? ''
                            )
                          : '已完成'}</span
                      >
                    </summary>
                  {/if}
                  <div class="diagnosis-process-body">
                    <div
                      class="diagnosis-live-timeline"
                      aria-label="诊断执行过程"
                    >
                      {#each processTimeline as item (item.id)}
                        {#if item.kind === 'analysis'}
                          {@const analysisText = item.text ?? ''}
                          {#key analysisText}<div
                              class="diagnosis-live-analysis diagnosis-markdown"
                            >
                              {@html renderMarkdown(analysisText)}
                            </div>{/key}
                        {:else if item.kind === 'investigation'}
                          {@const investigationKey = `process-investigation-${item.id}`}
                          <div class="diagnosis-investigation-wrap">
                            <button
                              class="diagnosis-live-action diagnosis-investigation"
                              aria-expanded={Boolean(
                                diagnosisActionExpanded[investigationKey]
                              )}
                              on:click={() => toggleExpanded(investigationKey)}
                            >
                              <span
                                class="diagnosis-live-action-chevron"
                                aria-hidden="true"
                                >{diagnosisActionExpanded[investigationKey]
                                  ? '⌄'
                                  : '›'}</span
                              ><span
                                class="diagnosis-live-action-icon"
                                aria-hidden="true"><SearchCheck size={13} /></span
                              ><span class="diagnosis-investigation-kind"
                                >子智能体</span
                              ><span class="diagnosis-live-action-label"
                                >{(item.label ?? '子智能体').replace(
                                  /^子智能体\s*·\s*/,
                                  ''
                                )}</span
                              ><span
                                class="diagnosis-live-action-status"
                                class:error={item.status === '失败'}
                                >{item.status}</span
                              >
                            </button>
                            {#if item.detail}
                              <p class="diagnosis-investigation-detail error">
                                {item.detail}
                              </p>
                            {/if}
                            {#if diagnosisActionExpanded[investigationKey]}
                              <div class="diagnosis-live-action-detail investigation-detail">
                                <div>
                                  <small>任务范围</small>
                                  <div class="diagnosis-live-action-value">
                                    <button
                                      class="diagnosis-live-action-copy"
                                      type="button"
                                      aria-label="复制任务范围"
                                      data-tooltip="复制任务范围"
                                      on:click|stopPropagation={() => copyDiagnosisAction(item.input ?? '暂无任务范围')}
                                      ><Copy size={13} /></button
                                    ><pre>{item.input ?? '暂无任务范围'}</pre>
                                  </div>
                                </div>
                                <div>
                                  <small>调查结果</small>
                                  <div class="diagnosis-live-action-value">
                                    <button
                                      class="diagnosis-live-action-copy"
                                      type="button"
                                      aria-label="复制调查结果"
                                      data-tooltip="复制调查结果"
                                      on:click|stopPropagation={() => copyDiagnosisAction(item.output ?? '暂无调查结果')}
                                      ><Copy size={13} /></button
                                    ><pre>{item.output ?? '暂无调查结果'}</pre>
                                  </div>
                                </div>
                              </div>
                              {#if item.actions?.length}
                                <div class="diagnosis-investigation-progress">
                                  <small>内部执行</small>
                                  {#each item.actions as progress (progress.id)}
                                    {#if progress.kind === 'analysis'}
                                      <div class="diagnosis-investigation-progress-note">
                                        {progress.text}
                                      </div>
                                    {:else}
                                      {@const progressKey = `process-investigation-action-${item.id}-${progress.id}`}
                                      <div class="diagnosis-live-action-wrap">
                                        <button
                                          class="diagnosis-live-action child"
                                          aria-expanded={Boolean(
                                            diagnosisActionExpanded[progressKey]
                                          )}
                                          on:click={() => toggleExpanded(progressKey)}
                                        >
                                          <span
                                            class="diagnosis-live-action-chevron"
                                            aria-hidden="true"
                                            >{diagnosisActionExpanded[progressKey]
                                              ? '⌄'
                                              : '›'}</span
                                          ><span
                                            class="diagnosis-live-action-icon"
                                            aria-hidden="true"><Wrench size={12} /></span
                                          ><span class="diagnosis-live-action-label"
                                            >{diagnosisActionLabel(progress)}</span
                                          ><span
                                            class="diagnosis-live-action-status"
                                            class:error={progress.status !==
                                              '已完成'}>{progress.status}</span
                                          ><small>{progress.duration}</small>
                                        </button>
                                        {#if diagnosisActionExpanded[progressKey]}
                                          <div class="diagnosis-live-action-detail child-detail">
                                            <div>
                                              <small>入参</small>
                                              <div class="diagnosis-live-action-value">
                                                <button
                                                  class="diagnosis-live-action-copy"
                                                  type="button"
                                                  aria-label="复制入参"
                                                  data-tooltip="复制入参"
                                                  on:click|stopPropagation={() => copyDiagnosisAction(progress.input ?? '暂无入参')}
                                                  ><Copy size={13} /></button
                                                ><pre>{progress.input ?? '暂无入参'}</pre>
                                              </div>
                                            </div>
                                            <div>
                                              <small>出参</small>
                                              <div class="diagnosis-live-action-value">
                                                <button
                                                  class="diagnosis-live-action-copy"
                                                  type="button"
                                                  aria-label="复制出参"
                                                  data-tooltip="复制出参"
                                                  on:click|stopPropagation={() => copyDiagnosisAction(progress.output ?? '暂无出参')}
                                                  ><Copy size={13} /></button
                                                ><pre>{progress.output ?? '暂无出参'}</pre>
                                              </div>
                                            </div>
                                          </div>
                                        {/if}
                                      </div>
                                    {/if}
                                  {/each}
                                </div>
                              {/if}
                            {/if}
                          </div>
                        {:else}
                          {@const actions = item.actions ?? [item]}
                          {#if actions.length > 1}
                            <div class="diagnosis-live-action-wrap">
                              <button
                                class="diagnosis-live-action summary"
                                aria-expanded={Boolean(
                                  diagnosisActionExpanded[
                                    `process-action-${item.id}`
                                  ]
                                )}
                                on:click={() =>
                                  toggleExpanded(`process-action-${item.id}`)}
                              >
                                <span
                                  class="diagnosis-live-action-chevron"
                                  aria-hidden="true"
                                  >{diagnosisActionExpanded[
                                    `process-action-${item.id}`
                                  ]
                                    ? '⌄'
                                    : '›'}</span
                                ><span
                                  class="diagnosis-live-action-icon"
                                  aria-hidden="true"><Wrench size={13} /></span
                                ><span class="diagnosis-live-action-label"
                                  >调用工具 · {actions.length} 个动作</span
                                ><span
                                  class="diagnosis-live-action-status"
                                  class:error={item.status !== '已完成'}
                                  >{item.status}</span
                                ><small>{item.duration}</small><small
                                  >总耗时 {item.elapsed}</small
                                >
                              </button>
                              {#if diagnosisActionExpanded[`process-action-${item.id}`]}
                                <div class="diagnosis-live-action-children">
                                  {#each actions as action (action.id)}
                                    {@const actionKey = `process-action-child-${action.id}`}
                                    <div
                                      class="diagnosis-live-action-child-wrap"
                                    >
                                      <button
                                        class="diagnosis-live-action child"
                                        aria-expanded={Boolean(
                                          diagnosisActionExpanded[actionKey]
                                        )}
                                        on:click={() =>
                                          toggleExpanded(actionKey)}
                                      >
                                        <span
                                          class="diagnosis-live-action-chevron"
                                          aria-hidden="true"
                                          >{diagnosisActionExpanded[actionKey]
                                            ? '⌄'
                                            : '›'}</span
                                        ><span
                                          class="diagnosis-live-action-icon"
                                          aria-hidden="true"
                                          ><Wrench size={12} /></span
                                        ><span
                                          class="diagnosis-live-action-label"
                                          >{diagnosisActionLabel(action)}</span
                                        ><span
                                          class="diagnosis-live-action-status"
                                          class:error={action.status !==
                                            '已完成'}>{action.status}</span
                                        ><small>{action.duration}</small>
                                      </button>
                                      {#if diagnosisActionExpanded[actionKey]}<div
                                          class="diagnosis-live-action-detail child-detail"
                                        >
                                          <div>
                                            <small>入参</small>
                                            <div class="diagnosis-live-action-value">
                                              <button
                                                class="diagnosis-live-action-copy"
                                                type="button"
                                                aria-label="复制入参"
                                                data-tooltip="复制入参"
                                                on:click|stopPropagation={() => copyDiagnosisAction(action.input ?? '暂无入参')}
                                                ><Copy size={13} /></button
                                              ><pre>{action.input ?? '暂无入参'}</pre>
                                            </div>
                                          </div>
                                          <div>
                                            <small>出参</small>
                                            <div class="diagnosis-live-action-value">
                                              <button
                                                class="diagnosis-live-action-copy"
                                                type="button"
                                                aria-label="复制出参"
                                                data-tooltip="复制出参"
                                                on:click|stopPropagation={() => copyDiagnosisAction(action.output ?? '暂无出参')}
                                                ><Copy size={13} /></button
                                              ><pre>{action.output ?? '暂无出参'}</pre>
                                            </div>
                                          </div>
                                        </div>{/if}
                                    </div>
                                  {/each}
                                </div>
                              {/if}
                            </div>
                          {:else}
                            {#each actions as action (action.id)}
                              {@const actionKey = `process-action-${action.id}`}
                              <div class="diagnosis-live-action-wrap">
                                <button
                                  class="diagnosis-live-action"
                                  aria-expanded={Boolean(
                                    diagnosisActionExpanded[actionKey]
                                  )}
                                  on:click={() => toggleExpanded(actionKey)}
                                  ><span
                                    class="diagnosis-live-action-chevron"
                                    aria-hidden="true"
                                    >{diagnosisActionExpanded[actionKey]
                                      ? '⌄'
                                      : '›'}</span
                                  ><span
                                    class="diagnosis-live-action-icon"
                                    aria-hidden="true"
                                    ><Wrench size={13} /></span
                                  ><span class="diagnosis-live-action-label"
                                    >{diagnosisActionLabel(action)}</span
                                  ><span
                                    class="diagnosis-live-action-status"
                                    class:error={action.status !== '已完成'}
                                    >{action.status}</span
                                  ><small>{action.duration}</small><small
                                    >总耗时 {action.elapsed}</small
                                  ></button
                                >
                                {#if diagnosisActionExpanded[actionKey]}<div
                                    class="diagnosis-live-action-detail"
                                  >
                                    <div>
                                      <small>入参</small>
                                      <div class="diagnosis-live-action-value">
                                        <button
                                          class="diagnosis-live-action-copy"
                                          type="button"
                                          aria-label="复制入参"
                                          data-tooltip="复制入参"
                                          on:click|stopPropagation={() => copyDiagnosisAction(action.input ?? '暂无入参')}
                                          ><Copy size={13} /></button
                                        ><pre>{action.input ?? '暂无入参'}</pre>
                                      </div>
                                    </div>
                                    <div>
                                      <small>出参</small>
                                      <div class="diagnosis-live-action-value">
                                        <button
                                          class="diagnosis-live-action-copy"
                                          type="button"
                                          aria-label="复制出参"
                                          data-tooltip="复制出参"
                                          on:click|stopPropagation={() => copyDiagnosisAction(action.output ?? '暂无出参')}
                                          ><Copy size={13} /></button
                                        ><pre>{action.output ?? '暂无出参'}</pre>
                                      </div>
                                    </div>
                                  </div>{/if}
                              </div>
                            {/each}
                          {/if}
                        {/if}
                      {/each}
                    </div>
                  </div>
                </svelte:element>
              {/if}
              <div class="diagnosis-bubble-f">
                {#if message.content}<div class="diagnosis-markdown">
                    {@html renderMarkdown(message.content, {
                      renderMermaid: !isStreaming || diagnosisAnswerCompleted
                    })}
                  </div>{/if}
                {#if message.role === 'user' && ((message.attachments?.length ?? 0) || (message.links?.length ?? 0))}
                  <div class="diagnosis-message-attachments">
                    {#each message.attachments ?? [] as attachment}<span
                        >{attachment.name}</span
                      >{/each}
                    {#each message.links ?? [] as link}<a
                        href={link}
                        target="_blank"
                        rel="noreferrer">{link}</a
                      >{/each}
                  </div>
                {/if}
              </div>
              {#if message.role === 'user'}
                <div class="diagnosis-user-actions">
                  {#if isLastDiagnosisUser(index)}<button
                      class="diagnosis-user-action"
                      aria-label="编辑并重新发送"
                      data-tooltip="编辑并重新发送"
                      on:click={(event) => beginMessageEdit(message, event)}
                      ><Pencil size={13} /></button
                    >{/if}
                  <button
                    class="diagnosis-user-action"
                    aria-label="复制问题"
                    data-tooltip="复制问题"
                    on:click={() =>
                      copyDiagnosisMessage(message, false, () =>
                        markMessageCopied(`question-${message.id}`)
                      )}
                    >{#if copiedMessageKey === `question-${message.id}`}<Check
                        size={13}
                      />{:else}<Copy size={13} />{/if}</button
                  >
                </div>
              {/if}
              {#if message.role === 'assistant' && (!isStreaming || diagnosisAnswerCompleted)}
                <div class="diagnosis-answer-actions">
                  <button
                    class="diagnosis-answer-copy"
                    aria-label="复制回答"
                    data-tooltip="复制回答"
                    on:click={() =>
                      isStreaming
                        ? copyStreamingAnswer(() =>
                            markMessageCopied('streaming-answer')
                          )
                        : copyDiagnosisMessage(
                            message,
                            diagnosisProcessExpanded[message.id] ?? false,
                            () => markMessageCopied(`answer-${message.id}`)
                          )}>{#if copiedMessageKey ===
                      (isStreaming ? 'streaming-answer' : `answer-${message.id}`)}<Check
                        size={14}
                      />{:else}<Copy size={14} />{/if}</button
                  >
                </div>
              {/if}
              {#if isStreaming && (diagnosisGenerating || (!diagnosisAnswerCompleted && diagnosisHasRunningActions(diagnosisSnapshot)))}
                <span
                  class="diagnosis-thinking diagnosis-thinking-inline"
                  aria-live="polite"
                  ><i></i><i></i><i></i><span>正在思考</span></span
                >
              {:else if isStreaming && !diagnosisAnswerCompleted && !diagnosisStreamingText && !processTimeline.length && !diagnosisInterruptedReason}
                <span class="diagnosis-thinking" aria-live="polite"
                  ><i></i><i></i><i></i><span>正在思考</span></span
                >
              {/if}
              {#if isStreaming && diagnosisShouldShowEmptyAnswer()}
                <span class="diagnosis-interruption"
                  ><i></i>模型未返回可展示的回答。</span
                >
              {/if}
            {/if}
          </div>
        </article>{/if}
    {/each}
    {#if interruptionReason}
      <div class="diagnosis-interruption standalone">
        <i></i><span>回答已中断：{interruptionReason}</span>
      </div>
    {/if}
  {/if}
</div>
