# I008 / I009 / I010 联合评审（跨迭代一致性与实施编排）

- **评审对象：**
  - [I008 Engine Runtime 适配与 Pi 接入](I008-engine-runtime-pi/iteration.md)（设计评审中）
  - [I009 专家组能力](I009-expert-groups/iteration.md)（设计评审中）
  - [I010 Skills 迁移与导入](I010-skills-import/iteration.md)（设计评审中）
- **评审范围：** 仅跨迭代层面（接口/契约冲突、一致性、依赖顺序、共享风险、总裁决）；单迭代内部细节由并行专项评审负责。
- **共同上游约束：** [docs/design/ai-engine.md](../design/ai-engine.md)（尤其 §19 红线）、[docs/design/llm-domains.md](../design/llm-domains.md)、[docs/standards/documentation.md](../standards/documentation.md)
- **评审日期：** 2026-11-18
- **证据基线：** 分支 feature/engine-expert-skills，commit edbeb96

---

## 1. 评审元信息

| 项 | 说明 |
|---|---|
| 评审方法 | 三份 iteration.md + R001 交叉比对，并抽查 backend/engine/types.go、backend/engine/runtime.go、backend/skill/service.go、backend/authorization/types.go、backend/migrations/sql、docs/design/ai-engine.md §14/§19 |
| 结论类型 | 总裁决 + 必须先裁定的跨迭代事项清单（见 §6/§7） |
| 局限 | 未运行测试；前端改动仅按设计文档与 ai-engine.md §14 页面清单比对，未逐页核对代码 |

---

## 2. 跨迭代冲突矩阵（改动点 × 三迭代）

| 共享改动点 | I008 | I009 | I010 | 冲突判定 |
|---|---|---|---|---|
| `backend/engine/types.go` Request 结构 | FR-002 契约冻结 + FR-004 改 `ModelBuildResult`（types.go:308-324 区域） | R001 §3.4：`Request` 增加 `ExpertGroupID`（types.go:169 附近） | 不涉及 | **顺序冲突（中）**。同一结构体的序列化契约被两边同时改动：I008 M1 要把 Request 冻结为基线快照，I009 T04 要加字段。新增 `omitempty` 字段不破坏既有事件序列，但 I009 若先合入，I008 的基线快照需包含 `expert_group_id` 路径；反之 I008 先合入基线，I009 T04 需补录含 ExpertGroupID 的契约用例。见 §6 裁定事项 1 |
| `backend/engine/runtime.go` 执行链 | 不改 runtime.go（FR-003 明确平台层逻辑不变） | T04 在 Persona 解析（runtime.go:212-231）前插入专家组解析 + `expert_group.resolved` 事件 | 不涉及 | **语义耦合（低-中）**。物理改动不冲突，但 I009 新增 `expert_group.resolved` 事件落在 I008 冻结的统一事件契约内。事件类型清单（ai-engine.md §19.5）需同步登记该事件，且 I008 契约测试的事件枚举需预留扩展位，否则 I009 合入即触发 I008 契约测试红灯 |
| `backend/engine/tools.go` PolicyGateway（tools.go:169） | FR-006：Pi 侧工具全部转发回 `PolicyGateway.Invoke`，不改 PolicyGateway 本身 | 不涉及 | 前置修复改 `backend/skill/service.go:255-262` `allowedToolName` | **无重叠（低）**。I008 改的是"调用路径"（runner.go / 新增 pi Adapter），I010 改的是"Skill 版本可声明的工具名集合"（skill 域校验函数）。但二者共同决定"技能最终能调用什么"：I010 T01 白名单补齐应先行且独立合入，I008 的工具审计契约测试（`engine_execution_tool_calls`）不受影响。注意 I010 R001 §1.3 证据属实：service.go:255-262 确实缺 oracle_/elasticsearch_/rabbitmq_/minio_ 前缀工具 |
| 执行选择快照 / 执行记录 | R001 §7：快照增加 runtime kind/version | 不涉及 | 不涉及 | 无冲突 |
| 数据库迁移（backend/migrations/sql，现最大编号 0003） | 0004 候选：engines 表 runtime 列 + 执行记录 runtime 快照列 | 需要：expert_groups / expert_group_members / expert_group_bindings 三新表 + inspection_policies 加列（合并为 1 个迁移） | 需要：skill_versions 加 source_type/source_digest 列 | **编号冲突风险（高，纯流程性）**。三迭代并行各自起分支，若各自以 0004 起号将无法合并。必须预先分配号段，见 §5.1 |
| Engine 目录 API（/api/v1/engines） | 增加 runtime 字段 | 不涉及 | 不涉及 | 无冲突 |
| 诊断/巡检输入 | 事件语义冻结、时间线不改 | StartInput/AppendMessageInput 增加 ExpertGroupID；巡检策略加 expert_group_id | 不涉及 | **潜在重叠（低）**：I008 验收标准 5 要求"执行记录可回答哪个 Runtime+模型"，I009 要求执行可回答哪个专家组；两者都动诊断执行的请求与记录展示，但字段互不碰撞。风险仅在诊断会话 UI 若两边同迭代改表单可能合并冲突 |
| 前端页面 | 模型页（Engine 目录，ai-engine.md §14） | 专家页 + 应用详情/诊断/巡检表单 | 技能页（导入向导） | **无重叠**。三页各自独立（§14 页面清单佐证），仅 I009 与 I008 在"诊断会话 Engine/专家组选择器"上有轻微共存，需 UI 排版协调，无逻辑冲突 |

---

## 3. 一致性检查结果

| 检查项 | 结论 | 证据 |
|---|---|---|
| 权限前缀风格 | ✅ 一致。I009 `expert_group:read/manage/use` 与既有 `skill:*`、`persona:*`、`engine:*`（backend/authorization/types.go:20-28）三段式一致；I010 复用 `skill:manage` 不新增 | authorization/types.go:20-28；I009 R001 §3.2；I010 R001 §3.2 |
| 表命名与模型风格 | ✅ 一致。I009 三表（expert_groups / members / bindings）沿用 0001_initial.sql 的 uuid 主键 + tenant_id/scope_id + 软删 + timestamptz 模式；I010 只加列不加表 | I009 R001 §3.1；I010 R001 §3.5 |
| scope 归属 | ✅ 一致。三者均坚持 `scope_id` 归属 + Scope RBAC 继承，不新增对象级授权表，对齐 llm-domains.md 既有模式 | I009 决策 1/4；I010 §4；I008 未引入新领域对象 |
| API 风格 | ✅ 一致。`/api/v1/expert-groups`、`/api/v1/skills/import` 均为资源式 REST + manage 守卫 + 审计，风格同 /api/v1/personas | I009 R001 §3.3；I010 R001 §3.4 |
| 版本化机制 | ✅ 一致。I009 成员引用 Persona 跟随最新 published 版本；I010 导入走既有 draft→publish 不可变版本。唯一注意点：I010 §2.3 提到 `opsseek.skill/v2` 可能性，属其内部演进，无跨迭代影响 | I009 R001 §2.1；I010 R001 §3.3 |
| 场景标签 | ✅ 一致。I009 绑定表复用 general/diagnosis/inspection/workflow 四值与 engine_scope_bindings 继承语义（backend/engine/catalog.go:85-101 模式确实存在，I009 引用属实） | I009 R001 §1.6/§3.1 |
| 事件契约 | ⚠️ 潜在不一致。I009 新增 `expert_group.resolved` 事件未在 I008 的冻结事件清单中出现；需在 I008 M1 契约文档中预留"平台生命周期事件"扩展类目并登记 | I008 决策 5 / R001 FR-002；I009 R001 §3.4 |
| 红线遵守 | ✅ 三份均未违反 ai-engine.md §19 红线：权限/预算/审计/事件格式留在 OpsSeek 外层；I010 fail-closed 不执行导入脚本，与 I005 非目标一致 | I008 §2/§4；I010 §4 |

---

## 4. 依赖与实施编排（推荐）

### 4.1 依赖图（文字版）

```
I010-T01 (白名单补齐, service.go)  ──独立、最小──→  可立即先行合入（不依赖任何迭代）
I008-M1 (契约冻结 + ADK 基线快照)  ──硬前置──→  I008-M2 (ADK Adapter 化) ──→ I008-M3 (Pi PoC) ──→ I008-M4 (双引擎)
I008-M1 ──软前置──→ I009-T04 (Engine 注入 ExpertGroupID)   [Request 契约变更应在基线之后]
I009-T01→T02→T03→T04→T05→T06 内部串行；T06 依赖 T03 即可，可与 T04/T05 并行开发
I010: T01 → {T02→T03→T06} 与 {T04→T05} 两条内部支线可并行
I008-M3/M4 与 I009-T04/T05、I010 全部支线互不依赖，可完全并行
```

### 4.2 推荐合并顺序

1. **第一批（立即）：** I010-T01 工具白名单补齐（backend/skill/service.go 纯前置修复，证据见 I010 R001 §1.3，本人复核属实）；同时 I008-M1 契约冻结合入 main（其退出标准本就要求"基线测试在 main 分支通过并合并"，I008 iteration.md §6 M1）。
2. **第二批：** I009-T01/T02/T03（纯新增领域，不碰 Engine）与 I010-T02/T03 并行推进；I008-M2 ADK Adapter 化随后。
3. **第三批：** I009-T04/T05（Request 加字段 + 事件登记，须在 I008-M1 基线之后，并补录含 ExpertGroupID 的契约用例）；I010-T04/T05/T06。
4. **第四批：** I008-M3 Pi PoC、M4 双引擎灰度（风险最高的部分最后做，且独立 feature flag，不阻塞其他两迭代发布）。

### 4.3 各迭代准入条件

| 迭代 | 进入实施前必须满足 |
|---|---|
| I008 | M1 契约文档 + 表格化测试 + ADK 基线快照先合入 main（这是 M2/M3 的合并门槛，也是 I009-T04 的前置）；§6 裁定事项 1/2 关闭 |
| I009 | I008-M1 已合入（Request 契约以何版本为基线）；Q1-Q5 中 Q2（worker_persona_ids 冲突）、Q3（巡检互斥）必须在 T04 编码前裁定，Q1/Q4/Q5 可延后；`expert_group.resolved` 事件在 I008 契约文档登记 |
| I010 | T01 白名单补齐先行合入；开放问题 4（MongoDB 工具命名）在 T05 前验证关闭；问题 1（fail-closed vs 映射向导）在 T02 前裁定 |

---

## 5. 共享风险与缓解

### 5.1 迁移编号与执行顺序（高风险，纯流程）

现状 `backend/migrations/sql` 最大编号 **0003**。三迭代各自至少 1 个迁移（I008：engines runtime 列 + 执行快照列；I009：三表 + inspection_policies 加列；I010：skill_versions 加列）。并行分支若都从 0004 起号，合并时必然冲突且顺序错乱。

**缓解（必须执行）：** 按推荐合并顺序预分配号段——I010-T01 无迁移；I008-M1 无迁移（纯测试），I008 首个迁移占 0004；I009 占 0005；I010 占 0006。若实际合入顺序变化，以后合入者重排号。所有迁移均为"只加表/加列/可空列、无数据回填"（三份设计均自述，且字段定义复核支持），顺序互换在 schema 层面安全，仅编号需唯一。

### 5.2 测试基线（中风险）

I008-M1 的 ADK 行为基线快照若晚于 I009-T04 / I010-T03 合入，则基线录制时已包含专家组与导入技能路径，出现"基线不纯"争议；反之若基线先行，I009/I010 合入后需确认基线测试仍绿（I009 加 `omitempty` 字段 + 新事件类型若被基线断言严格匹配会误报）。

**缓解：** 基线断言对 Request 序列化采用"字段存在性白名单"而非整串 diff；事件断言按类型枚举登记制，新增 `expert_group.resolved` 走登记而非改基线。

### 5.3 Engine 配置/目录兼容（低风险）

I008 给 engines 加 runtime 配置、给执行快照加 runtime 字段，均为增量；I009 不碰 Engine 目录。唯一注意：I008 FR-008 的"场景标签互斥与向上继承"与 I009 绑定继承都依赖 catalog.go 的同一查询模式，I009 复制该 SQL 模式（`WITH RECURSIVE ... row_number()`）到 expert_group_bindings 时应抽公共 helper 或保持两处实现一致并加对照测试，避免后续一处修 bug 一处漏。

### 5.4 发布节奏（低-中风险）

三迭代同分支产出，若打包为一次发布，I008 的 Pi（Node ≥22.19 子进程、镜像变更）会绑架 I009/I010 的发布。**缓解：** Pi Adapter 以 feature flag 独立开关（I008 R001 §9 已含），建议 I009/I010 在 I008-M2 完成后即可随常规节奏发布，不等 M3/M4。

---

## 6. 总裁决

**结论：需先解决跨迭代问题（有条件批准）——三份设计方向自洽、边界清晰、未发现不可调和的架构冲突，但在以下事项裁定前不应同时进入实施。**

三份设计构成连贯演进路线：I008 夯实执行层契约（使后续任何 Runtime/请求扩展有稳定基座）→ I009 在契约之上叠加配置层编队（明确"不是新调度器"，复用委派与域过滤）→ I010 扩充技能供给侧（不动 Engine 边界）。三者对 llm-domains.md 领域边界、§19 红线的遵守一致。核心矛盾只有一处实质性技术耦合（Request 契约 + 事件契约的先后），其余均为流程性（迁移编号、基线时点）。

## 7. 必须先裁定的跨迭代事项清单

| # | 事项 | 影响 | 建议 |
|---|---|---|---|
| 1 | **I008-M1 契约冻结是否作为 I009-T04 的硬前置合入门槛**（Request 加 `ExpertGroupID` vs 基线快照谁先） | 三迭代唯一的结构性顺序决策 | 是。I008-M1 先行合入 main；I009-T04 在其上扩展并补契约用例。基线断言采用字段白名单避免 I009 误伤 |
| 2 | **`expert_group.resolved` 事件的契约登记方式**：纳入 I008 冻结事件清单，还是定义为可扩展的"平台生命周期事件"类目 | 决定 I009 合入是否触发 I008 契约测试变更 | 定义扩展类目（类似 `context.compacted` 的常量模式，types.go:277），I009 只登记不改 I008 基线 |
| 3 | **迁移编号预分配**（0004-0006 归属） | 合并冲突 | 按第 5.1 节分配，写入各自 R001 实施说明 |
| 4 | **I010-T01 白名单补齐是否剥离为独立小 PR 立即合入** | 它同时是 I010 前置修复和 I009/I008 无关的存量缺陷（Oracle/ES/RabbitMQ/MinIO 域技能当前无法创建） | 剥离先行，不等 I010 整体评审完成 |
| 5 | **I009-Q2（lead 的 worker_persona_ids 与组 assistant 取舍）与 Q3（巡检互斥）** | 影响 I009-T04/T05 的 Engine 与巡检改动形态，需在 I008 契约之上一次性定案 | 同意 I009 现设计（以组为准、忽略 lead 自带列表；巡检二选一校验拒绝），但须由用户显式确认 |
| 6 | **scope 继承查询模式是否抽公共 helper**（catalog.go vs expert_group_bindings 两处 `WITH RECURSIVE`） | 长期维护一致性 | 建议抽 helper，作为 I009-T01 验收项之一，不阻塞批准 |

---

*本文件为联合评审产出，未修改任何设计文档与代码。*
