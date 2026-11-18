# I008 Engine Runtime 适配与 Pi 智能体框架接入

**迭代编号：** I008
**目录标识：** I008-engine-runtime-pi
**状态：** 设计评审中
**计划开始：** 2026-11-18
**计划封板：** 待定

## 1. 迭代目标

落实 [ai-engine.md §19](../../design/ai-engine.md#19-多-agent-runtime-候选方案) 已确认的 Adapter-first 多引擎架构：冻结 `engine.Engine` 执行契约，把当前直接内嵌在 `backend/engine/runner.go` 的 Google ADK Agent Loop 重构为第一个 Runtime Adapter（行为等价），并以 Pi 开源智能体框架（badlogic/pi-mono，现仓库 earendil-works/pi）作为第二个 Runtime Adapter 完成 PoC，最终实现 ADK 与 Pi 双引擎按 Scope 共存、可配置、可回滚。

本迭代不改变业务层（诊断、巡检、技能、工作流）对 Engine 的依赖方式；权限、工具策略、上下文解析、预算、取消、审计和事件格式继续全部由 OpsSeek 外层控制，任何 Runtime 不得接管（红线见 §2）。

## 2. 已确认的设计决策

1. **Adapter-first（§19.3 方案 A）。** 上层 `engine.Engine`、`Request/Result/Event`、Tool Gateway 和事件 Store 契约冻结不变；每个框架实现统一 `Runner`/`StreamingRunner`（或更底层的 `EngineAdapter`），由 Engine 配置的 `runtime.kind` 选择适配器。
2. **ADK 为默认与回滚基线。** 现有 ADK 链路重构为 `adk` Adapter 后必须先通过行为等价回归（相同 Mock Model + 工具 Fixture + Request 下事件序列、预算、取消、审计逐项对齐基线快照），才允许注册第二个 Runtime。
3. **Pi 为第二个 Runtime 适配器。** Pi（TypeScript 生态，`@earendil-works/pi-agent-core` + `pi-ai`，MIT，Node ≥ 22.19）通过其 RPC 模式（JSONL stdin/stdout）以独立子进程集成，Go 后端不引入 Node 进程内依赖；推荐架构选型详见 [R001 §6](R001-requirement.md)，最终以评审结论为准。
4. **模型解析框架无关化。** `ModelBuildResult.Client model.LLM`（`backend/engine/types.go:309`）携带的 ADK 专属句柄需扩展为统一模型描述/调用端口：Provider/Model、协议、能力、限制和一次性内存凭据由 OpsSeek 解析，各 Adapter 自行构造模型客户端，凭据不落入适配器持久化状态。
5. **事件归一化。** Pi 的 `agent_start/turn_start/message_update/tool_execution_start/.../agent_end` 事件必须映射为 OpsSeek 统一事件（`execution.started`、`model.started`、`assistant.delta/progress`、`tool.requested/started/completed/failed`、`model.resumed`、`execution.*` 终态，见 ai-engine.md §19.5），框架事件不得直接写入平台 Store。
6. **工具网关唯一入口。** Pi 的 `beforeToolCall`/工具执行必须路由回 OpsSeek `PolicyGateway`（`backend/engine/tools.go:169`）；Pi 自带的 bash/edit 等内置工具、文件系统访问和 MCP 直连在适配器中一律禁用或包装为 Tool Gateway 转发。
7. **版本锁定。** Pi Adapter 注册时固定已验证的 npm 包版本（如 `@earendil-works/pi-agent-core@1.5.2` 或经核验的锁定版本），版本写入适配器注册表和执行选择快照。
8. **Pi 的 Session/扩展/Skill 体系不作为平台主数据。** 仅可作为非权威内部实现，执行终态、工具许可、可见性始终由 OpsSeek 决定。

## 3. 范围

- 契约冻结：Request/Result/Event/工具调用/取消/预算/能力矩阵的接口文档与表格化契约测试；当前 ADK 行为基线快照。
- ADK Adapter 化：把 `backend/engine/runner.go` 中 ADK 创建、回调、事件转换抽入 `adk` Adapter，平台层保留上下文、策略、审计和生命周期。
- 框架无关模型入口：Provider 解析结果扩展为统一模型描述，保留 ADK 兼容路径。
- Pi Adapter PoC：子进程 RPC 集成、事件映射、Tool Gateway 转发、取消、预算、结构化输出；实验性 Engine 注册，仅测试 Scope 或显式选择。
- 双引擎共存验证：相同 Mock Model + Fixture 的一致性/故障测试；Engine 目录 `runtime.kind` 配置与前端能力展示。
- 迁移与回滚策略（按 Scope 灰度，默认关闭自动回退）。

## 4. 非目标

- 不引入 Ingenimax、Covonaut、Eino、Microsoft Agent Framework Go 等其他候选依赖（保留在 §19 候选清单）。
- 不把 Pi 设为任何场景标签的默认 Engine；不改变现有 ADK 诊断、巡检、技能、工作流生产链路行为。
- 不允许任何 Runtime 接管权限、Scope 鉴权、工具审批、预算、审计事件格式或执行主状态。
- 不启用 Pi 的 bash/文件系统/浏览器等内置工具和 MCP 直连作为平台工具来源。
- 不引入 Redis/Kafka 等新默认依赖；不做 Provider/Model 层改动（多厂商接入已由 `backend/llm` 完成）。
- 不在本迭代内决定 Pi 是否长期保留为生产 Runtime（由 PoC 验收与后续迭代决定）。

## 5. 风险与依赖

| 风险 | 说明 | 缓解 |
|---|---|---|
| TS 运行时部署 | 镜像需内嵌 Node ≥ 22.19 与 npm 锁定依赖，体积与供应链审计成本上升 | 独立精简 Node 运行时层；npm lockfile 固定；PoC 阶段仅测试环境部署 |
| 子进程生命周期 | 长连接、崩溃恢复、僵尸进程、并发上限（当前 `maxConcurrentExecutions=8`，`backend/engine/runtime.go:25`） | 执行级子进程、进程池上限、健康检查与强制回收 |
| 流式事件转换 | Pi JSONL 事件粒度（`text_delta`、`agent_settled`）与 OpsSeek 事件语义不一致；U+2028/U+2029 分帧陷阱 | 适配器内严格 LF 分帧解析；契约测试锁定事件序列 |
| 取消与超时传递 | Go context 取消需转换为 Pi 进程终止；半途终止的审计完整性 | `agent_end`/stdin 关闭协议 + 兜底 SIGKILL；终态幂等测试 |
| 凭据边界 | 模型 API Key 传入 Pi 进程的暴露面 | 一次性内存凭据、进程环境隔离、禁止 Pi 侧持久化 Session |
| 工具网关安全 | Pi 直连工具/MCP 旁路风险 | 适配器只注册 Tool Gateway 转发工具；安全测试覆盖旁路尝试 |
| 行为等价回归 | ADK Adapter 化可能改变既有事件/预算语义 | 基线快照 + 表格化契约测试先行，作为合并门槛 |

**依赖：** I003 Engine 契约与审计、I004 统一资源接入的 Tool Gateway/RBAC；Pi 上游 MIT 许可与版本锁定；部署环境可运行 Node 子进程。

## 6. 里程碑

| 阶段 | 名称 | 内容 | 退出标准 |
|---|---|---|---|
| M1 | 契约冻结 | 契约文档、表格化测试、ADK 行为基线快照 | 基线测试在 main 分支通过并合并 |
| M2 | ADK Adapter 化 | runner.go 拆分 adk Adapter + 框架无关模型入口 | 行为等价回归全绿，用户行为不变 |
| M3 | Pi 适配器 PoC | 子进程 RPC、事件映射、Tool Gateway 转发、取消/预算 | 相同 Fixture 下契约/权限/故障测试通过 |
| M4 | 双引擎共存验证 | `runtime.kind` 配置、实验性 Engine 注册、按 Scope 灰度方案 | 双引擎共存测试与回滚演练通过，进入验收评审 |

## 7. 需求清单

| 需求 | 名称 | 任务 | 状态 | 需求文档 | 验收报告 |
|---|---|---:|---|---|---|
| R001 | Runtime Adapter 契约与 Pi 集成方案 | 待分解 | 设计评审中 | [R001-requirement.md](R001-requirement.md) | 待补充 |

## 8. 封板记录

待补充。
