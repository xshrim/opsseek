# R001 专家组领域与应用绑定 — 设计评审报告

- **评审对象：** docs/iterations/I009-expert-groups/iteration.md、R001-requirement.md
- **评审日期：** 2026-11-18
- **评审人：** 设计评审智能体（I009）
- **评审性质：** 严格、基于证据的技术评审
- **结论：** **有条件通过**（必改项 3 条，见 §7）

---

## 1. 事实核验结果

对设计文档引用的关键代码位置逐一抽查（共 13 处）：

| # | 文档引用 | 核验结果 | 说明 |
|---|---|---|---|
| 1 | backend/persona/service.go:26-35（Persona 结构：scope_id/identifier/…/status） | ✅ 属实 | 与 `Persona` 结构体一致 |
| 2 | backend/persona/store.go:29-35、211-233（版本草稿/发布/停用） | ✅ 属实 | `PersonaVersionStore` 接口含 draft/published/disabled 语义（status CHECK 见 migrations 0001:1397） |
| 3 | backend/persona/service.go:257-265（Resolve 优先 published，NotFound 回退主表 config） | ✅ 属实 | `GetPublishedPersonaVersion` 成功则覆盖 configSource，ErrNotFound 时保留主表 config |
| 4 | backend/persona/service.go:360-396（manager 1-10 worker、worker 不得有 WorkerPersonaIDs、manager 不得持有资源工具） | ⚠️ 基本属实，一处偏差 | "worker 必须有 domain"不完全准确：`validatePersonaConfig` 仅在 `explicitRole`（显式声明 `role=worker`）时要求 domain（service.go:369）；config 未写 role 时默认按 worker 处理但不强制 domain。设计文档如依赖"worker 必有 domain"做成员校验，需按真实规则重述 |
| 5 | backend/authorization/types.go:26-28（persona:read/manage/use） | ✅ 属实 | |
| 6 | backend/httpapi/skill.go:204-231（/api/v1/personas 路由风格） | ✅ 待验证（未逐行核对，路由模式与既有包风格一致性由 T03 落地时复核） | 标记待验证 |
| 7 | backend/engine/types.go:169（Request.PersonaID） | ✅ 属实 | |
| 8 | backend/engine/runtime.go:212-231（ResolvedPersona + persona.resolved 事件）、234-237 / 252-261（worker 域收窄与资源过滤） | ✅ 属实 | 过滤逻辑实际位于 252-265，文档行号 252-253/355-361 在容差范围内；另确认 worker（无 domain 时）`RestrictKinds=true` 但 `AllowedKinds` 不设值（236-238），见 §3 主要问题 M2 |
| 9 | backend/diagnosis/investigation.go:178-198（manager 展开 WorkerPersonaIDs 为 expert_N；worker 作为主专家直接返回空）、:248（域不匹配拒绝） | ✅ 属实 | |
| 10 | backend/inspection/store.go:65（inspection_policies.persona_id 单值可空）、types.go:33 | ✅ 属实 | |
| 11 | backend/application/types.go:11（ProjectScopeID）、:41（Instance.TargetResourceID）、:54-58（Dependency.TargetResourceID/dependency_kind/binding） | ✅ 属实 | |
| 12 | backend/diagnosis/orchestrator.go:417（ApplicationContext → ResourceIDs/ResourceBindings/LocatorHints 注入） | ✅ 属实 | |
| 13 | backend/engine/catalog.go:85-101（WITH RECURSIVE + row_number 按 depth 继承）；migrations 0001:953-961（engine_scope_bindings 四值 tag CHECK） | ✅ 属实 | 注意：既有 `engine_scope_bindings` **无主键约束**（见 953-961），设计新表却引入表达式主键，见 §3 次要问题 m1 |
| 14 | migrations 0001:1401（personas 表） | ✅ 属实 | |
| 15 | docs/design/llm-domains.md 权限/场景标签章节、ai-engine.md §3.6/§9/§12 | ✅ 属实 | 独立权限前缀、不新增对象级授权表、四值场景标签 + general 兜底语义均与文档一致 |

**总评：现状描述基本可信，未发现虚构引用；仅 #4 存在规则表述偏差、#6 待落地复核。**

---

## 2. 领域建模专项意见

### 2.1 与既有领域风格的一致性（正面确认）

- `expert_groups` 表结构（tenant_id/scope_id/identifier/UNIQUE(scope_id,identifier)/deleted_at）与 `personas`（0001:1401-1413）逐字段对齐；identifier 不编码归属信息 ✅。
- 独立权限前缀 `expert_group:read/manage/use` 与 `persona:*`（authorization/types.go:26-28）模式一致，不新增对象级授权表，符合 llm-domains.md §权限 ✅。
- 绑定表复用 scope 继承查询模式（catalog.go:85-101），与 `engine_scope_bindings` 语义对齐 ✅。
- 复用 Application 关联资源作为专家上下文、不复制 Resource 数据的边界划分正确（orchestrator.go:417 链路确实已完备）✅。

### 2.2 席位制与 manager 内嵌 worker 的语义冲突

设计（R001 §2.4 + Q2）已给出裁定方向：**以组成员为准，lead 自带 WorkerPersonaIDs 忽略**。但存在未闭环点：

- investigation.go:179-200 的专家展开完全以"传入 personaID 的 Role"为分支（worker 直接返回空，manager 展开内嵌列表）。改为"lead + assistant 集合"后，assistant 成员若本身是 manager，其 WorkerPersonaIDs 是否再展开？R001 §2.1 说"assistant 成员展开为其 WorkerPersonaIDs（或本身为 worker）"——这实际引入了**两层展开**，与 §2.1 末尾"复用既有单层委派、不是新调度器"的表述存在张力。若 assistant 也是 manager，则形成 lead → assistant(manager) → assistant 的 worker，超出"单层委派"承诺。**建议明确：assistant 席位只接受 worker 角色 Persona，manager 类专家只能担任 lead**（见必改项 B2）。
- lead 席位定义为"必须是 manager 角色或无域约束专家"含糊。runtime.go:234-238：`ResolvedPersona.Role == "worker"` 即触发 `RestrictKinds`，若 lead 解析为 worker（即使无 domain），整个诊断会话的上下文都会被收窄，这与 lead 应"总览全部关联资源"的语义直接冲突。**建议：lead 强制要求 manager 角色，管理端与解析时双重校验。**

### 2.3 场景标签与绑定继承的一致性问题

- **general 兜底缺失（主要问题 M1）**：llm-domains.md §41 明确 Engine/Provider 绑定"没有场景专用绑定时由 general 绑定兜底"。R001 §2.2 的解析优先级只写了"应用级 → 项目 → 团队 → 平台"，未声明 expert_group_bindings 是否同样按 `diagnosis → general` 标签兜底。若不兜底，"给应用配一个 general 组"将永远不生效，前端语义也会与 Engine 绑定不一致；若兜底，优先级表达式应为"应用级(diagnosis) → 应用级(general) → scope 链(diagnosis) → scope 链(general)"还是"标签优先、层级其次"必须写明。**当前留白会导致实现歧义。**
- 成员 `scene_tags` 限定为 diagnosis/inspection/workflow 子集，绑定表却允许 general：允许空 scene_tags=全场景可以覆盖 general，逻辑自洽，但"general 绑定 + 成员仅标 diagnosis"时该成员不参与 general 场景——组合语义未在文档中给出算例。建议补一个解析真值表。
- 应用级绑定存储 `scope_id + application_id`：应用所属项目迁移/换 scope 后绑定成为孤儿；建议绑定写入时以 `ProjectScopeID` 冗余并校验，解析时复核应用与绑定 scope 仍匹配，否则降级走 scope 链（当前文档只写了创建时校验，未写解析时复核）。

### 2.4 数据模型细节

- `expert_group_members` 无 `updated_at`、无 `deleted_at`，PUT 整体替换成员是物理删除，可接受但与主表软删风格不一致，建议注明理由。
- "成员 Persona 必须与本组同 scope 链可见（解析时校验）"：解析时校验意味着配置期可以引用一个当前不可见的 Persona，错误暴露延迟到执行失败，与 §3.4.4"不静默降级"叠加后用户体验为"巡检半夜失败"。**建议至少在保存成员时对授权 scope 做一次可见性校验，执行期再复核。**
- 每组最多 1 个 lead、成员 ≤20 为应用层约束，文档未说明是否加 DB CHECK（lead 唯一性可加 partial unique index），建议补充实现要求。

---

## 3. 执行链专项意见

- **上下文注入**：设计明确专家组解析只决定 Persona 集合，资源上下文仍走 ApplicationContext（§2.3），与 runtime.go:240-266 现状吻合，闭环 ✅。
- **版本固定**：跟随最新 published 版本与单 Persona 行为一致（service.go:257-265），但**巡检可复现性矛盾未裁定**（Q1 悬置）。巡检策略是 cron 定时执行，同一策略两次运行可能因 Persona 重新发布而行为漂移，且 `expert_group.resolved` 事件若只在执行事件流里记录版本，历史对比需要跨执行聚合。至少要求：执行审计（engine_execution_events / 巡检 run 记录）持久化"专家组 ID + 每成员 persona_id + 解析出的 version"快照。
- **审计记录专家组信息**：R001 §3.4.1 只提了 `expert_group.resolved` 事件，对齐 `persona.resolved`（runtime.go:231）；但诊断会话表/巡检策略表是否落 `expert_group_id` 及解析快照未写明。诊断会话目前逐消息取 `latestMessage.PersonaID`（orchestrator.go:417），引入 ExpertGroupID 后：会话中途成员被编辑，同一会话前后轮专家集合不一致，是否符合预期需要显式声明（建议：会话首条消息解析后冻结成员集合快照到会话，后续轮次复用）。
- **Engine 请求级解析的归属层问题（主要问题 M2）**：Runtime 在 Persona 解析前解析专家组（§3.4.1），但 Runtime 无 Application 概念，应用级绑定解析只能发生在 diagnosis/inspection 调用方；而 scope+场景默认解析放在 Runtime 还是调用方，文档未拆清。若 Runtime 也做 scope 链解析，则 engine 包需新增 expertgroup 依赖，与 ai-engine.md §9"engine 不依赖 Skill/Diagnosis 等"的分层约束冲突（engine 目前通过 personaResolver 接口注入，专家组宜同样以接口注入，需写明）。
- **巡检互斥（Q3）与 persona_id 并存**：见 §5 裁定建议。
- **失败语义**：§3.4.4"按现状 invalid 错误返回"✅，但未覆盖"组被停用后仍被绑定引用""lead 解析失败但 assistant 可用"等分支——应明确任一必需成员（lead）失败即整体失败，且停用组等价于无绑定（返回未配置行为）还是报错，二选一写死。

---

## 4. 需求完备性与可测性

- iteration.md §7 退出条件偏描述性；R001 无独立"验收标准"小节。建议为 R001 补充可测条目，至少覆盖：绑定继承四级解析、成员域过滤行为、无专家组回归不变、专家组停用行为、审计含成员版本快照。
- **遗漏场景清单**（需在需求文档补充处理规则）：
  1. 成员 Persona 被删除/软删/停用（Resolve 报 not active，service.go:251-253）——配置期预警 + 执行期报错的分工；
  2. 跨 scope 引用 Persona（解析期 `allowsScope` 拒绝，service.go:254-256）——见 §2.4 校验前移建议；
  3. 空专家组 / 无 lead 组（保存时应拒绝，需写明）；
  4. 并发：成员 PUT 整体替换与进行中执行并发（执行前快照即可，需声明）；两组对同一 scope+tag+application 绑定的写入竞争（主键已保证唯一，OK）；
  5. 组停用但绑定仍指向该组（解析语义未定义，见 §3）；
  6. 诊断 AppendMessage 逐消息指定 PersonaID 与会话 ExpertGroupID 并存时的优先级（请求级写了"无显式 PersonaID"，会话级未写）。

---

## 5. 开放问题裁定建议

| 问题 | 推荐裁定 | 理由 |
|---|---|---|
| Q1 成员是否锁定 Persona 版本 | **首版跟随最新 published 版本，不引入 pinned_version_id**；可复现性以"执行审计持久化解析版本快照 + 巡检 run 记录绑定 version"保障；pinned 列入 backlog | 与单 Persona 行为一致（service.go:257-265），避免首版引入版本引用完整性与"引用版本被停用"新错误类；审计快照已能满足事后追溯 |
| Q2 与 manager 内嵌 worker 列表冲突 | **以组成员为准，lead 自带 WorkerPersonaIDs 忽略**；管理端在 lead 为 manager 且带 worker 列表时给出显著提示；同时收紧 assistant 仅允许 worker 角色 | 与"配置层编队"定位一致；避免两层展开破坏单层委派承诺（见 §2.2）。忽略必须在校验与文档双重显式化，防止用户误以为内嵌列表生效 |
| Q3 巡检 persona_id 与 expert_group_id 并存 | **校验拒绝（互斥），创建/更新策略时同时设置即报错** | "persona 优先"属于静默语义，巡检是定时无人值守场景，静默优先会造成配置误解且难排查 |
| Q4 oncall 席位 | **首版仅预留枚举值（DB CHECK 含 oncall），不实现时间段路由**；API 对 oncall 拒绝或标记实验性 | 值班路由需要时间窗模型与成员排期数据，超出本迭代边界；预留枚举避免后续迁移 |
| Q5 平台级内置专家组 | **首版不随发布包安装内置专家组** | 内置 Skill 是通用契约资产，而专家团队强依赖各组织的资源领域与 Persona 资产，平台预置组大概率无人可用（成员引用不存在的 Persona）；待真实运营数据沉淀后再评估 |

---

## 6. 分级问题清单

### 阻塞（必改后方可批准实施）

- **B1** 绑定解析优先级未声明 general 兜底及"标签 vs 层级"次序（§2.3 M1），实现歧义直接影响 T04。
- **B2** lead/assistant 角色约束含糊："无域约束专家"任 lead 会触发 runtime.go:234-238 的 worker 域收窄作用于全会话；assistant 为 manager 时造成两层展开。必须改为：lead 仅允许 manager 角色，assistant 仅允许 worker 角色（§2.2）。
- **B3** 解析职责分层未定义：应用级绑定解析必须在 diagnosis/inspection 调用方完成，Runtime 侧解析是否注入接口（对齐 personaResolver 模式）需写明，避免 engine 包直接依赖 expertgroup（§3 M2，依据 ai-engine.md §9 分层约束）。

### 主要

- M1（=B1）见上。
- M2（=B3）见上。
- **M4** 巡检/诊断的审计与快照要求不完整：未要求持久化"组+成员+解析版本"快照，未定义会话中途成员变更的冻结策略（§3）。
- **M5** 遗漏场景未覆盖：组停用仍被绑定、无 lead/空组保存、AppendMessage PersonaID 与 ExpertGroupID 并存优先级（§4）。
- **M6** "worker 必须有 domain"的表述与实际校验不符（service.go:369 仅显式 worker 强制），文档需修正，且成员校验规则须按真实规则重写（§1 #4）。

### 次要

- m1 `expert_group_bindings` 表达式主键 `COALESCE(application_id, zero-uuid)` 在 PG 可行但非项目惯例（engine_scope_bindings 无主键，0001:953-961）；且新表缺 `tag CHECK` 约束（既有表有，0001:960），建议补齐对齐。
- m2 `expert_group_members` 无 updated_at/软删，与主表风格不一致，建议注明取舍。
- m3 lead 唯一、成员 ≤20 建议给出 DB 层实现要求（partial unique index）。
- m4 应用级绑定在应用换项目/scope 后的孤儿处理（解析时复核）未写明。
- m5 成员保存期 scope 可见性校验建议前移（§2.4）。
- m6 R001 缺验收标准小节（§4）。

---

## 7. 评审结论与必改项

**结论：有条件通过。**

事实核验通过（13/13 关键引用属实，1 处规则表述偏差），领域建模方向与既有风格一致，复用既有委派链与资源上下文的策略正确。但存在 3 条阻塞问题（B1-B3），必须在设计文档修订后复审通过方可进入 T01 实施。

**必改项（阻塞项，改后需复审）：**
1. 补齐绑定解析的 general 兜底与完整优先级表达式（含真值表示例）。
2. 明确席位角色约束：lead=manager、assistant=worker，并写明 Q2 裁定的忽略规则与提示。
3. 划清解析职责：应用级解析在调用方、Runtime 通过注入接口解析 scope 默认组，保持 engine 分层约束。

**建议项（主要 M4-M6、次要 m1-m6）随必改项一并修订，不单独阻塞。**

开放问题 5 条的裁定建议见 §5，建议在修订版中直接落定并关闭。
