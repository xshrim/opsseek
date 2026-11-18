# I008 / R001 设计评审报告

| 项 | 内容 |
|---|---|
| 评审日期 | 2026-11-18 |
| 评审对象 | `docs/iterations/I008-engine-runtime-pi/iteration.md`、`R001-requirement.md` |
| 评审依据 | `docs/design/ai-engine.md` §6/§8/§10/§19；`docs/design/llm-domains.md`；代码抽查（backend/engine、backend/llm、backend/cmd）；Pi 上游公开文档复核（earendil-works/pi rpc.md、npm @earendil-works/pi-agent-core） |
| 评审方法 | ① 对设计文档引用的文件:行号逐一抽查；② 对照 §19 红线逐条符合性检查；③ 需求可测性与遗漏场景分析；④ 对 Pi RPC/事件协议事实做外部复核（WebSearch，标注来源） |
| 评审结论 | **有条件通过**（必改项 3 项，见 §6） |

---

## 1. 事实核验结果

### 1.1 代码位置抽查（14 项）

| # | 文档引用 | 抽查结论 | 依据 |
|---|---|---|---|
| 1 | `runner.go:16-24` 直接 import ADK v2（agent/llmagent/model/runner/tool/functiontool）+ genai + jsonschema 库 | ✅ 属实 | 实测 16-17 行为 `adkjsonschema`/`tekurischema`，18-24 行为 `google.golang.org/adk/v2/...` 五个包 + `genai` |
| 2 | `runner.go:569-574` `llmagent.New` 构造 ADK Agent | ✅ 属实 | 569 行起 `llmagent.New(llmagent.Config{...})`，注入 Tools/OnToolErrorCallbacks/BeforeModelCallbacks |
| 3 | `runner.go:578` `runner.NewInMemory` | ✅ 属实 | 578 行 |
| 4 | `runner.go:669-888` 事件主循环迭代 `adkRunner.Run` | ✅ 属实 | 669 行起 `for event, eventErr := range adkRunner.Run(ctx, ...)`，循环内直接消费 `genai.Content/Part/FunctionCall/FunctionResponse/UsageMetadata/Partial` |
| 5 | `runner.go:432-507` `toolErrorCallback` 依赖 `agent.Context`/`tool.Tool` | ✅ 属实 | 432 行签名 `func(toolCtx agent.Context, failedTool tool.Tool, ...)` |
| 6 | `runner.go:508-568` `beforeModel` 改写 `model.LLMRequest` | ✅ 属实 | 508 行起 `func(_ agent.Context, req *model.LLMRequest)`，含 `req.Config.MaxOutputTokens`（566） |
| 7 | `runner.go:946-1179` `agentTools`（read_observation 952、上下文工具 1019、内部工具 1119、`ToolGateway.Invoke` 1075） | ✅ 属实 | 946 行 `func (r *AgentRunner) agentTools(...)`；952 行 `read_observation`；1019/1119 行 `functiontool.New` 包装；1075 行 `request.ToolGateway.Invoke(toolCtx, call)` |
| 8 | `runner.go:1202-1241`、`1656-1736` `functionResponseObservation`/`trimContext` | ✅ 属实 | 1202 行起 `functionResponseObservation`；1656 行起 `trimContext(req *model.LLMRequest, window int)` |
| 9 | `runner.go:1738-1743` `streamingMode` 映射 | ✅ 属实 | 1738 行 `func streamingMode(stream bool) agent.StreamingMode` |
| 10 | `runner.go:726-741` Thought 私有推理不外泄 | ✅ 属实 | 726-741 行注释与代码："Thought parts are model-private reasoning..." |
| 11 | `types.go:289-302` `Runner/StreamingRunner/Engine`、`types.go:308-324` `ModelBuildResult.Client model.LLM` | ✅ 属实 | 290/294/297-302 行接口定义；309 行 `Client model.LLM` |
| 12 | `runtime.go:25` `maxConcurrentExecutions = 8`；`runtime.go:64` 起 `Runtime.Execute` | ✅ 属实 | 25 行常量；64 行 `func (r *Runtime) Execute` |
| 13 | `tools.go:22/32/39/169` `ToolDefinition/ToolResult/Tool/PolicyGateway`；`retry_model.go:11-12/32-39/219`；`llm/service.go:270` `BuildModel`；`chatcompletions.go:30` | ✅ 属实 | 各行号均对上（tools.go PolicyGateway 结构体在 169 行附近；retry_model.go 219 行 `var _ model.LLM = (*retryingModel)(nil)`；service.go 270 行 `func (s *Service) BuildModel(...) (ResolvedProvider, model.LLM, error)`） |
| 14 | `cmd/api/main.go:230`、`cmd/worker/main.go:105` 生产实例仅一个 Runtime | ✅ 属实（轻微偏差） | 两处均为 `engine.NewWithContextAndStore(engine.NewAgentRunner(modelBuilder), ...)`，单 Runtime 描述成立；行号精确到 ±2 行级别，无实质偏差 |

**小结：14 项抽查全部属实，未发现虚构引用。** 文档对"未被 ADK 污染的部分"（runtime.go/tools.go/context.go/catalog.go/store.go/authorization.go/budget.go）的判断经抽查与 §19.3 方案 A 的前提一致。

### 1.2 设计文档事实偏差

| 引用 | 结论 |
|---|---|
| R001 §1.1 "`Request`（types.go:155-189）" | 属实（Request 结构体 155-189 行） |
| R001 §1.1 "`Result`（types.go:277-287）" | 待验证（本次抽查确认了同区间 Event 位于 262-269；Result 行号未逐字段核对，风险低） |
| iteration.md "1700+ 行文件" | 属实（runner.go 共 1743 行） |

### 1.3 Pi 上游事实复核（WebSearch，2026-11-18）

| 文档声称 | 复核结论 | 来源 |
|---|---|---|
| RPC 模式为 JSONL、严格 LF 分帧、ID 关联 | ✅ 属实（stdin 命令/stdout response/会话事件/扩展 UI 四类记录，命令支持异步 `id` 关联） | earendil-works/pi `packages/coding-agent/docs/rpc.md` |
| `agent_settled` 为完成信号（`prompt` 成功 ≠ 完成；`agent_end` 后仍可能有 retry/compaction/steering） | ✅ 属实 | 同上 |
| `pi --mode rpc --no-session` 启动 | ✅ 属实 | 同上 |
| 事件族 `agent_start→turn_start→message_start/update/end→tool_execution_start/update/end→turn_end→agent_end` | ✅ 属实 | npm `@earendil-works/pi-agent-core` README |
| `beforeToolCall/afterToolCall` 钩子存在 | ✅ 属实，**但文档未提关键细节：`beforeToolCall` 在 `tool_execution_start` 之后、参数校验之后才触发**（见问题 P2） | npm `@earendil-works/pi-agent-core` README |
| `@earendil-works/pi-agent-core@1.5.2` | ✅ 1.5.2 为当前最新版（约 2026-11-11 发布） | npmjs.com |
| Node ≥ 22.19 | ⚠️ 待验证（本次未直接核到 engines 字段原文，M3 前应以 package.json `engines` 实测为准） | — |
| 关闭 stdin 优雅退出 | ⚠️ 待验证（rpc.md 摘要确认进程由 stdin/stdout 控制，但"关闭 stdin = 优雅退出"的精确语义未在本次摘要中逐字确认；取消路径见问题 P3） | — |
| "Pi RPC 面向单控制端，不直接服务多并发多租户"（方案二缺点） | ✅ 公允（stdin/stdout 单控制器模型） | rpc.md |

---

## 2. 红线符合性检查（ai-engine.md §19）

| 红线 | 设计响应 | 结论 |
|---|---|---|
| 权限/Scope 鉴权不由框架接管 | FR-001：Adapter 只接收"已完成授权的不可变快照"；§19.4 `AdapterDeps` 对齐 | ✅ 符合 |
| Tool Gateway 唯一入口，禁 MCP 直连/内置工具 | iteration §2.6、FR-006、非目标第 4 条 | ✅ 符合 |
| 预算/取消由外层控制 | FR-007（适配器内"强制执行"预算——语义上是执行外层下发的预算值，可接受，但需在 FR 措辞上明确预算**判定权**在平台，见 P-minor-3） | ✅ 基本符合 |
| 审计事件格式归 OpsSeek，框架事件不直接入 Store | iteration §2.5、U4 | ✅ 符合 |
| 凭据边界：适配器不得接收原始 API Key 可变句柄 | FR-004/NFR-001（一次性内存凭据） | ✅ 符合 |
| 执行主状态/终态由 OpsSeek 决定 | iteration §2.8、§19.5 状态归属 | ✅ 符合 |
| 默认 Runtime 为 ADK、可回滚、执行中不切换、默认关闭自动回退 | NFR-004、§9 迁移与回滚 | ✅ 符合（对齐 §19.6 规则） |
| 能力归一化，未验证不点亮 | §8 前端影响 | ✅ 符合 |
| 版本锁定写入注册表与执行快照 | iteration §2.7、§7 | ✅ 符合 |

未发现红线违反。

---

## 3. 分级问题清单

### 阻塞（无）

### 主要（P1-P4，均对应必改项或需在封板前解决）

**P1（需求遗漏）：子进程在容器环境的沙箱与资源约束未成需求。**
NFR-001 仅提"受限用户运行"。Pi 子进程将执行不可信模型产出的工具参数转发（尽管经 Gateway），且自带 bash/edit 内置工具的禁用依赖适配器配置而非进程级隔离。缺少：cgroup/CPU/内存上限、seccomp/capability 收敛、进程组（`Setpgid`）管理防止 SIGKILL 漏杀孙进程、只读根文件系统 + 临时工作目录、stdout/stdout 管道背压（Pi 工具结果若大于管道缓冲 ~64KB 且 Go 侧未并发排水，双向 JSONL 会死锁——这是 JSONL 子进程集成的经典故障）。依据：FR-005/FR-006 未覆盖；U3 只覆盖崩溃/超时。**必改：NFR 增补进程级资源/沙箱/管道并发读写要求，并纳入验收 4 的故障注入矩阵。**

**P2（开放问题 3 实际可裁定，不应挂起）：**上游文档证实 `beforeToolCall` 在 `tool_execution_start` **之后**触发。但这不阻塞时序合成：由于 Pi 侧工具全部是适配器注册的转发桩，桩体完全受控——`tool_execution_start`（含 toolName/args）映射 `tool.requested`；桩在收到 Go 侧 `PolicyGateway.Invoke` 授权通过并开始执行时发 `tool.started`；`tool_execution_end` 映射 `tool.completed/failed`。该规则确定性强，设计应在 FR-006 直接固定，而非留作开放问题。依据：pi-agent-core README 事件流 + `tools.go:169` PolicyGateway。

**P3（协议细节缺口）：取消语义未定义到协议级。**FR-007 写"stdin 关闭/进程终止兜底"，但 rpc.md 语义是 stdin 关闭≈会话结束；Go context 取消应优先映射为 RPC 级中断命令（若 rpc.md 提供 interrupt/abort 类命令——待验证），stdin 关闭为二级、SIGKILL 为三级兜底。同时缺"取消后 Pi 侧已发出的 `tool_execution_end` 与平台终态幂等合并"规则（iteration §5 提及终态幂等测试，但需求未定义事件去重/乱序合并规则）。**必改：FR-007 分层取消协议 + 终态合并规则写入需求。**

**P4（遗漏场景）：Go 后端崩溃时的孤儿 Pi 进程。**验收 4 只测"Pi 子进程崩溃"方向；反向（API/Worker 重启后 Pi 进程残留、继续持有凭据与环境变量中的模型 Key、可能继续回写无人消费的 stdout）未覆盖。需进程组 + 启动握手 PID 上报 + 启动时清扫（如 stale pidfile/socket）设计。依据：NFR-001/NFR-003 均未提及。

### 次要

- **P-minor-1**：FR-009 假定 Pi 支持 OutputSchema 等价校验；pi-ai 结构化输出能力**待验证**。若不支持，需明确降级策略（Go 侧终态校验 + 失败重试或直接拒绝）。
- **P-minor-2**：Node ≥ 22.19 与 `--no-session` 行为待在 M3 前以 package.json/rpc.md 原文逐字核验（见 §1.3）。
- **P-minor-3**：FR-007"预算在适配器内强制执行"措辞易误读为预算判定权下放；建议改为"适配器执行平台下发的预算并只产生规范终止事件"。
- **P-minor-4**：开放问题 4（隐藏推理）应规定安全默认：无法区分 reasoning/text 时一律丢弃该 delta（对齐 `runner.go:726-741` 的拒绝暴露语义）。
- **P-minor-5**：`maxConcurrentExecutions=8` 与进程池上限的关系（每执行一进程则进程数=槽位；池化则需独立上限与排队策略）应在 FR/NFR 明确，避免双重限流死锁。
- **P-minor-6**：R001 §1.1 称 Result 位于 types.go:277-287，待逐字段核对（低风险）。

---

## 4. 需求完备性与可测性

- FR-001~FR-010 均可测；FR-002/FR-010 的"表格化契约测试 + 相同 Mock Model/Fixture"是本设计最强的一环，与 §19.6 第 1/5 步严格对齐。
- 验收 1-6 可测，覆盖事件序列对齐、审计 100% 经 Gateway、四类故障终态、默认链路不变、回滚等价。
- 遗漏：P1（容器沙箱/资源/管道背压）、P4（孤儿进程）、P-minor-1（结构化输出降级）、SSE 断线续读与子进程生命周期的交互（验收 4 提及"断线续读正常"但未定义客户端断线 ≠ 执行取消时 Pi 进程继续运行的用例——目前语义是断线续读不取消执行，Pi 链路下需确认事件在进程存活期间持续入 Store，建议补一条显式用例）。

## 5. 选型论证评估

- 三方案对比公允：方案一的优缺点与 rpc.md 事实一致；方案二"单控制端、需自研多租户协议"的批评有据；方案三"非接入 Pi"的定性准确。
- 推荐理由（侵入最小、进程隔离、PoC 可整体删除）成立，且与 §19.3"方案 C 作为后备隔离部署"的定位一致。
- 保留意见：方案一到方案二的升级触发条件（冷启动/内存阈值）只有定性描述，建议 PoC 退出标准量化（如单执行冷启动 >X ms 或 Node 常驻 RSS >Y MB 才评估池化/sidecar），避免演进路径含糊。

## 6. 开放问题裁定建议

| # | 问题 | 推荐裁定 | 理由 |
|---|---|---|---|
| 1 | 模型调用绕过 pi-ai vs 自定义 provider 注入 | **自定义 provider 注入统一模型端口**（同文档推荐）。若 pi-ai 无 custom provider 扩展点（待验证），fallback：适配器起本地 loopback OpenAI-compatible endpoint 指向 Pi，凭据仍只存在于 Go 侧 | 凭据单向、Provider 域不旁路、审计归一；pi-ai 直连会让 Provider 解析/能力校验出现第二套实现 |
| 2 | 每执行一进程 vs 常驻池 | **PoC 阶段每执行一进程** | 生命周期与执行对齐、崩溃隔离、无跨执行状态泄漏；`agent_settled`/steer 的会话态复用与常驻池需要额外状态机，属过早优化。量化冷启动数据后再议 |
| 3 | tool.requested/started 时序合成 | **可直接裁定**（见 P2）：`tool_execution_start`→`tool.requested`；转发桩获 Gateway 授权并开始执行→`tool.started`；`tool_execution_end`→`tool.completed/failed` | 工具体是适配器受控转发桩，合成点确定；不应留作开放问题 |
| 4 | message_update 思考/文本区分是否足够 | **裁定规则：可区分则映射，不可区分则丢弃 delta**（安全默认），并在契约测试中锁定 | 对齐 runner.go:726-741 拒绝暴露隐藏推理的既有语义；"宁可丢流不可泄推理" |
| 5 | read_observation 在 Pi 侧等价实现 | **注册为平台转发桩工具**，与上下文工具同一机制；observation 索引/分页状态由适配器持有（对齐 executionState 语义），不依赖 Pi 内置能力 | iteration §2.8 已定 Pi Session 非权威；分页状态属平台执行状态 |
| 6 | Node 引入方式与体积预算 | **独立 Docker layer + npm ci 锁定 + 多阶段构建的精简 node 运行时**；体积预算给出量化上限（建议镜像增量 ≤150MB，PoC 报告实测）；与方案二（sidecar 容器）的边界预先写明 | 供应链可控、可整体 feature flag 禁用；预先定义边界防止"进程内池化"滑向无设计的 sidecar |

## 7. 评审结论

**有条件通过。** 设计与 §19 方案 A 高度一致，代码引用经 14 项抽查全部属实，未发现虚构；红线全部符合；选型论证有据。

**必改项（封板前）：**
1. P1：补容器内子进程沙箱/资源上限/进程组/管道背压需求与故障注入用例。
2. P3：FR-007 定义分层取消协议与终态/乱序事件合并规则。
3. P2：开放问题 3 按 §6 裁定直接写入 FR-006，移出开放问题清单。

**建议项（不阻塞）：** P4 孤儿进程清扫、P-minor-1 结构化输出降级、P-minor-4 隐藏推理安全默认、方案升级阈值量化、Node 版本/rpc.md 取消命令原文核验（标注待验证的 4 处）。
