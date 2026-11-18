# R001 验收记录：Runtime Adapter 契约与 Pi 集成

**所属迭代：** [I008](iteration.md) | **验收日期：** 2026-11-18 | **状态：** 实施完成（待运行环境验收）

## 交付范围与证据

| 项 | 交付 | 证据 |
| --- | --- | --- |
| FR-001 EngineAdapter 接口 | 框架无关接口、AdapterRequest 不可变快照+Validate、AdapterDeps（ToolInvoker/EventSink/审计/取消）、注册表、能力闸门、冻结事件目录与扩展前缀登记 | commit 3671d66 `backend/engine/adapter.go` |
| FR-002 契约冻结与基线 | 表格化契约测试（字段白名单/事件目录/工具生命周期/预算/能力矩阵）+ ADK 行为基线快照 6 场景 | 3671d66 `adapter_contract_test.go`、`adapter_baseline_test.go` |
| FR-003 ADK Adapter 化 | runner.go 不再 import ADK；ADK 实现收敛 `adapter_adk.go`；工厂 `runtime_factory.go`（`OPSK_ENGINE_RUNTIME` 默认 adk，未知回退留痕）；基线 6 场景零修改通过 | commit 4c32ec6；`go test ./engine/ -count=1` 146 passed |
| FR-004 框架无关模型入口 | ModelBuildResult 增 Protocol/Description/ModelCredential(json:"-")；`Client model.LLM` 兼容字段保留（基线测试仍构造它，移除条件未达成，按设计预案记录） | 3671d66 types.go、adapter.go |
| FR-005~FR-007、FR-011~FR-013 Pi 适配器 | `adapter_pi.go`：执行级子进程（Setpgid+PID 握手）、JSONL 严格 LF 分帧、事件映射（agent_settled 终态、thinking 丢弃、转发桩合成 tool.requested→started）、三级取消（interrupt→关 stdin→SIGKILL 进程组）、迭代/工具/Token/输出/超时预算、孤儿登记表+启动清扫（ReapPiOrphans）、stdin 写入超时与输出上限防背压、凭据仅环境变量单向注入 | commit 9e4326e；`adapter_pi_test.go` fake 子进程 12 模式（正常 ReAct/未知工具/失败/取消实杀/超时/崩溃/三类预算/U+2028 分帧/256KB 背压/孤儿回收/凭据不泄露） |
| FR-008/FR-009 runtime 配置 | 迁移 0004（engines.runtime_kind/runtime_version/experimental）、API 与 catalog 贯通、per-Engine runtime 解析、`OPSK_ENABLE_PI_RUNTIME` feature flag（默认不注册生产）、ExperimentalAdapter 标记 | 9e4326e；migrations_test 6 项有序断言 |
| FR-010 双引擎一致性 | pi fake 子进程序列与 ADK 冻结基线 assertBaselineSequence 逐项一致 | 9e4326e `adapter_pi_test.go` |
| 前端 | 模型页 Engine 目录 runtime 标识/版本/实验性徽标 | 9e4326e `LLMPage.svelte`、`api.ts` |

## 验证结果

- `go build ./...` 通过；`go vet ./engine/...` 无问题。
- `go test ./engine/... ./migrations/... ./httpapi/... -count=1`：332 passed。
- 后端全量 `go test ./... -count=1`：877 passed / 2 failed（仅 `tool/repository` 环境性 TLS 既有问题，与本迭代无关）/ 1 skipped（真 Pi 集成测试按 OPSK_PI_INTEGRATION 守护跳过）。
- 前端 `npm run check` 0 errors、`npm run build` ✓、`npm test` 105 passed。

## 未尽事项与待验证

1. 真 Pi 子进程联调（需 Node ≥ 22.19 运行环境与固定版本 npm 包）属运行环境验收，测试已就位但默认跳过。
2. Node 运行时镜像层与部署方案（Dockerfile 未改动，按设计留待部署迭代）。
3. rpc.md `interrupt` 命令字面核验（设计待验证项②）。
4. `TestRuntimeBoundsConcurrentExecutionsAndHonorsQueueCancellation` 在机器高负载下会抖动失败（既有问题，单跑稳定通过）——建议后续单独加固。
