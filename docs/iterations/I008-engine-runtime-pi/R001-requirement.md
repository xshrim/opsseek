# R001 Runtime Adapter 契约与 Pi 集成方案

**所属迭代：** [I008](iteration.md) | **状态：** 设计评审中

## 1. 现状与问题

### 1.1 现状

- Engine 已有统一契约：`engine.Engine`（`backend/engine/types.go:297-302`）、`Runner`/`StreamingRunner`（`types.go:289-295`）、`Request`/`Result`/`Event`（`types.go:155-189`、`277-287`、`262-269`）、`EventSink`、预算、取消与事件持久化由 `Runtime.Execute`（`backend/engine/runtime.go:64-346`）统一控制。
- Provider 层多厂商接入已完成（`backend/llm`：openai_compatible/anthropic/gemini/deepseek/qwen/kimi/glm/doubao 等），`llm.Service.BuildModel` 返回 `model.LLM`（`backend/llm/service.go:270`），OpenAI 兼容实现为 `ChatCompletionsModel`（`backend/llm/chatcompletions.go:30`）。
- 生产实例只有一个 `Runtime`，默认 `AgentRunner` 把 Google ADK Go 的 Agent Loop 直接接入 runner（`backend/cmd/api/main.go:230`、`backend/cmd/worker/main.go:105`；现状描述另见 `docs/design/llm-domains.md` §Engine 运行时适配现状）。

### 1.2 ADK 耦合点清单（代码位置）

| 位置 | 耦合内容 |
|---|---|
| `backend/engine/runner.go:16-24` | 直接 import `google.golang.org/adk/v2`（agent/llmagent/model/runner/tool/functiontool）与 `google.golang.org/genai`、ADK 依赖的 jsonschema 库 |
| `backend/engine/runner.go:569-574` | `llmagent.New` 构造 ADK Agent，注入工具与回调 |
| `backend/engine/runner.go:578` | `runner.NewInMemory` 创建 ADK InMemory Runner，Agent Loop 主循环由 ADK 接管 |
| `backend/engine/runner.go:669-888` | 事件主循环迭代 `adkRunner.Run`，直接消费 ADK 的 `genai.Content/Part/FunctionCall/FunctionResponse/UsageMetadata/Partial` 等框架事件结构 |
| `backend/engine/runner.go:432-507` | `toolErrorCallback` 依赖 `agent.Context`、`tool.Tool`（ADK 类型），在 ADK 工具生命周期内重建 OpsSeek 工具审计事件 |
| `backend/engine/runner.go:508-568` | `beforeModel` 回调直接改写 ADK `model.LLMRequest`/`genai.GenerateContentConfig`（温度、输出上限、SystemInstruction 注入、上下文裁剪） |
| `backend/engine/runner.go:946-1179` | `agentTools` 用 `functiontool.New` + ADK jsonschema 把 OpsSeek 工具包装为 ADK 工具（`read_observation` 952、上下文工具 1019、内部工具 1119）；工具调用入口 `ToolGateway.Invoke` 在 1075 |
| `backend/engine/runner.go:1202-1241`、`1656-1736` | `genai.FunctionResponse` 解析、`trimContext` 直接操作 `model.LLMRequest`/`genai.Content` |
| `backend/engine/runner.go:1738-1743` | `streamingMode` 映射 ADK `agent.StreamingMode` |
| `backend/engine/types.go:12`、`308-324` | `ModelBuildResult.Client model.LLM`（ADK 专属模型句柄）与 `ModelBuilder` 签名 |
| `backend/engine/retry_model.go:11-12`、`32-39`、`219` | `retryingModel` 实现并包装 ADK `model.LLM` |
| `backend/llm/service.go:270-350`、`chatcompletions.go:30` | Provider 解析层同样以 `model.LLM` 为返回类型 |

未被 ADK 污染的部分：`runtime.go`（生命周期/并发/事件持久化）、`tools.go`（`ToolDefinition`/`ToolResult`/`Tool`/`PolicyGateway`，`tools.go:22/32/39/169`）、`context.go`（`ResolvedContext`/`ContextResolver`，`context.go:30/75`）、`catalog.go`、`store.go`、`authorization.go`、`budget.go` 均为框架无关——这是 Adapter-first 方案可直接落地的基础。

### 1.3 问题

1. `runner.go` 中"平台策略（预算/审计/事件）"与"ADK 框架细节"交织在同一 1700+ 行文件中，无法在不影响事件语义的前提下替换 Agent 框架。
2. `ModelBuildResult.Client model.LLM` 使任何非 ADK Runtime 都拿不到模型调用端口（ai-engine.md §19.4 已指出该问题）。
3. 无 `runtime.kind` 概念：Engine 目录（`POST /api/v1/engines`，见 llm-domains.md）无法表达"由哪个框架驱动"，无法注册第二个 Runtime。
4. 用户期望引入 Pi（badlogic/pi-mono）生态的 Agent 能力，但其为 TypeScript 运行时，与 Go 后端的进程模型、事件流、凭据与工具边界需要明确的集成架构决策。

## 2. 用户场景

- **U1（平台管理员）**：在模型页 Engine 目录创建 `runtime.kind=pi` 的实验性 Engine，绑定固定 Pi 版本，查看其能力词条；平台/团队/项目场景默认仍指向 ADK Engine。
- **U2（诊断用户）**：在测试 Scope 显式选择 Pi Engine 发起诊断，前端时间线（阶段摘要→工具动作→Observation→下一轮分析）与 ADK Engine 呈现一致语义。
- **U3（开发/运维）**：Pi 子进程崩溃或超时后，执行以明确的失败/取消事件收尾，事件可断线续读，无僵尸进程、无重复工具副作用。
- **U4（安全审计）**：无论哪个 Runtime，工具调用全部落入 `engine_execution_tool_calls`，入参/出参/状态/耗时/错误齐全且脱敏；Pi 尝试调用未注册工具被拒绝并留痕。

## 3. 功能需求

| 编号 | 需求 | 说明 |
|---|---|---|
| FR-001 | Runtime Adapter 接口 | 定义框架无关的 `EngineAdapter`（含 ID/Version/Capabilities/Execute/Stream），Adapter 只接收已完成授权的不可变快照与 `ToolInvoker`/`EventSink`/取消/审计回调（对齐 ai-engine.md §19.4 草案） |
| FR-002 | 契约冻结与基线 | Request/Result/统一事件/工具调用/取消/预算/能力矩阵形成表格化契约测试；当前 ADK 行为录制为基线快照 |
| FR-003 | ADK Adapter 化 | `runner.go` 的 ADK 创建、回调、事件转换迁入 `adk` Adapter；平台层（runtime.go）逻辑不变；行为等价回归全绿 |
| FR-004 | 框架无关模型入口 | `ModelBuildResult` 扩展统一模型描述（Provider/Model/协议/能力/上下文窗口/输出上限/温度/一次性凭据），各 Adapter 自建模型客户端；保留 `model.LLM` 兼容字段直至双 Adapter 契约测试通过 |
| FR-005 | Pi 适配器（子进程 RPC） | Go 侧 `pi` Adapter 通过 JSONL stdin/stdout 驱动 Pi 子进程；事件（`agent_start`…`agent_end`、`text_delta`、`tool_execution_*`）映射为统一事件；`prompt` 后以 `agent_settled` 判定完成 |
| FR-006 | 工具网关转发 | Pi 侧工具全部由适配器注册为 Tool Gateway 转发桩：Pi 发起工具调用→适配器转 `PolicyGateway.Invoke`→结果回写 Pi；禁用 Pi 内置 bash/edit/MCP 直连 |
| FR-007 | 取消/预算/超时传递 | Go context 取消→stdin 关闭/进程终止兜底；迭代、工具数、Token、输出、超时预算在适配器内强制执行并产生规范终止事件 |
| FR-008 | Engine 目录 runtime 配置 | Engine 实例支持 `runtime: {kind, version, experimental}`；同一 Scope 场景标签互斥与向上继承规则不变；执行选择快照记录 runtime 与版本 |
| FR-009 | 结构化输出与能力校验 | Pi Engine 同样支持 OutputSchema 校验；能力矩阵不满足的请求被拒绝并说明缺失能力 |
| FR-010 | 双引擎一致性测试 | 相同 Mock Model、工具 Fixture、Request 下，adk/pi 两个 Adapter 的事件顺序、权限拒绝、工具纠错、超时、取消、终态幂等逐项对比 |

## 4. 非功能需求

- **NFR-001 安全**：凭据仅内存传递、不落 Pi 持久化状态；Pi 子进程以受限用户运行；所有工具调用经 Tool Gateway 审计；适配器不得接收原始 API Key 的可变句柄。
- **NFR-002 可观测**：适配器层结构化日志（进程 PID、版本、事件映射失败计数）；执行事件含 runtime kind/version。
- **NFR-003 资源**：Pi 子进程池受 `maxConcurrentExecutions` 约束（runtime.go:25），进程内存/启动时间有上限；镜像增量可控并记录。
- **NFR-004 兼容与回滚**：默认 Runtime 保持 ADK；`runtime.kind` 配置可一键回退；已发生模型输出或工具副作用的执行不自动切换 Runtime（§19.6 规则）。
- **NFR-005 可维护**：Pi npm 依赖 lockfile 固定、版本锁定写入注册表；上游破坏性变更在升级评审中处理。

## 5. 验收标准

1. FR-002 基线契约测试合入并在 main 通过；ADK Adapter 化后同套测试无需修改语义即通过（行为等价）。
2. 测试 Scope 下创建 Pi Engine 并显式选择执行诊断：SSE 事件序列与 ADK 基线在事件类型、顺序、关键字段上对齐（表格逐项比对）。
3. Pi 工具调用 100% 经 `PolicyGateway`，`engine_execution_tool_calls` 记录完整；直连/未注册工具调用被拒绝且产生 `tool.failed` 事件。
4. 取消、超时、各预算上限、进程崩溃四类故障下，执行产生规范终态事件，无僵尸进程、无事件丢失、断线续读正常。
5. 同一 Scope 场景标签只解析到一个默认 Engine；执行记录可回答"哪个 Runtime 的哪个版本 + 哪个 Provider 的哪个模型"。
6. 默认链路（未显式选择 Pi 的所有执行）与改造前完全一致；回滚到仅 ADK 注册表后系统行为不变。

## 6. Pi 集成架构选型对比

Pi 背景（调研于 2026-11，来源见 §10）：badlogic/pi-mono 现为 earendil-works/pi，MIT，TypeScript monorepo（Node ≥ 22.19），含 `pi-ai`（多厂商统一 LLM API）、`pi-agent-core`（有状态 Agent：`agent_start→turn_start→message_start/update/end→tool_execution_start/update/end→turn_end→agent_end` 事件流，`beforeToolCall/afterToolCall`、`prepareRequest/finishTurn` 钩子，steer/followUp）、`pi-coding-agent`（四种使用模式：交互 TUI、print/JSON、RPC、TS SDK 内嵌）、MCP 原生支持；无内置权限/沙箱体系。RPC 模式为 JSONL（严格 LF 分帧，注意 U+2028/U+2029 陷阱）、ID 关联、`agent_settled` 完成信号、关闭 stdin 优雅退出。

### 方案一：子进程 + RPC JSONL 协议（推荐）

每次（或池化）执行启动 `pi --mode rpc --no-session` 子进程，Go 适配器以 JSONL 命令驱动、消费会话事件并映射为统一事件；工具通过适配器注册的转发桩回到 Tool Gateway；模型配置由适配器以一次性凭据注入（Pi 的 `pi-ai` 多厂商 API 可直接对接现有 Provider 协议）。

- 优点：语言无关、进程隔离（崩溃不拖垮 Go 服务）、协议有官方文档与语义（`agent_settled`、事件族）、凭据与会话生命周期随进程结束、与 §19.3 方案 C 的"后备隔离部署"方向一致。
- 缺点/风险：镜像需 Node 运行时（体积、供应链）；JSONL 分帧解析陷阱；取消→进程终止的语义要在 Go 侧兜底；长连接池管理；事件粒度映射工作量（`tool_execution_*` → `tool.requested/started/completed` 需拆分合成）。

### 方案二：独立 Pi Sidecar 服务（Node 常驻，HTTP/gRPC 内部协议）

将 Pi 以常驻 Node 服务部署，OpsSeek Go 后端通过内部 API 提交任务、回流传 events（SSE/WebSocket）。

- 优点：避免每执行冷启动；依赖与故障域独立；便于独立扩缩容。
- 缺点：需自研任务提交/事件回传/取消/多租户隔离协议（Pi RPC 是面向单控制端的，不直接服务多并发多租户）；长连接事件回传与 OpsSeek 断线续读 Store 的对接复杂；凭据常驻 sidecar 内存扩大暴露面；运维多一个有状态服务。§19.3 已指出 Sidecar 成本高，仅作后备。

### 方案三：仅借鉴设计，Go 自研 Agent Loop（不引入 Pi）

把 Pi 的最小化 Agent Loop、事件模型和 Codemode 思想在 Go 侧重写为 `opsseek-native` Adapter。

- 优点：零 TS 运行时成本，完全同进程。
- 缺点：不是"接入 Pi"，无法利用其生态（扩展、MCP、pi-ai 多厂商适配）；等于新增一个自维护框架，与用户"优先支持 Pi"的目标不符。

**推荐：方案一（子进程 + RPC）**。它对现有 Go 架构侵入最小、进程隔离天然匹配 §19 红线，且 PoC 失败时可直接删除而不留部署负担。若 PoC 证明冷启动/资源开销不可接受，再评估升级为方案二。

## 7. Engine 配置与数据模型影响

- Engine 目录增加 `runtime_kind`/`runtime_version`/`experimental`（对齐 §19.4 示例 JSON）；适配器注册表为代码内静态注册（`adk`、`pi`），不落库。
- 执行选择快照（执行记录）增加 runtime kind/version，满足可追溯验收（§19.6 规则 7）。
- 迁移：新增迁移只加列，不改既有 ADK 执行数据；回滚保留默认 `adk`。

## 8. API 与前端影响

- API：`POST/PATCH /api/v1/engines` 增加 runtime 字段；执行响应/事件 Payload 增加 `runtime` 元信息；无破坏性变更。
- 前端：模型页 Engine 目录展示 Runtime 标识、版本与实验性标记及能力词条（归一化中文词条，未验证能力不点亮）；诊断/巡检时间线不改——事件语义冻结是前提；Engine 选择器允许显式选择已授权且能力兼容的 Pi Engine。
- Worker（`backend/cmd/worker/main.go:105`）与 API 使用同一 Runtime Factory。

## 9. 迁移与回滚策略

1. M1 基线先行，M2 ADK Adapter 化保持默认行为；合并门槛为契约测试全绿。
2. Pi Adapter 仅实验性注册：只允许测试 Scope 或显式 `engine:use` 选择，不允许设为任何场景默认。
3. 灰度：通过契约测试后按团队/项目 Scope 灰度，上级 ADK 默认保留为回退配置。
4. 回滚：取消 Pi Engine 注册或将场景标签指回 ADK Engine 即可，无数据迁移；Pi 子进程层可整体禁用（feature flag）。
5. 执行中不切换 Runtime；默认关闭自动回退，回退仅允许发生在模型调用开始前且记录原因（§19.6）。

## 10. 开放问题

1. Pi 模型调用是否完全绕过 `pi-ai`、由适配器把 OpsSeek 统一模型端口以自定义 provider 形式注入？（推荐后者，凭据单向）
2. Pi 进程池策略：每执行一进程 vs 常驻池（涉及冷启动 ~Node 进程启动耗时实测）。
3. `tool_execution_*` 与 OpsSeek `tool.requested→tool.started` 的时序合成规则（Pi 是否暴露"工具被模型请求"与"开始执行"的可区分信号）。
4. Pi `message_update` 中思考/文本区分是否足以执行现有"不暴露隐藏推理"策略（runner.go:726-741 同等语义）。
5. 观察分页（`read_observation`）在 Pi 侧的等价实现方式。
6. Node 运行时在目标部署镜像中的引入方式与体积预算。

## 附：调研来源

- [earendil-works/pi（原 badlogic/pi-mono）](https://github.com/earendil-works/pi)
- [pi RPC 协议文档 rpc.md](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/rpc.md)
- [@earendil-works/pi-agent-core](https://www.npmjs.com/package/@earendil-works/pi-agent-core)、[@earendil-works/pi-coding-agent](https://www.npmjs.com/package/@earendil-works/pi-coding-agent)
- [pi.dev](https://pi.dev)、[pi-mcp-extension](https://pi.dev/packages/pi-mcp-extension)
