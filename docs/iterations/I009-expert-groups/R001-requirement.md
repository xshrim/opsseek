# R001 专家组领域与应用绑定（需求分析与方案设计）

> **版本：** v2（设计返修稿，2026-11-18）
> **返修依据：** [R001-design-review.md](R001-design-review.md)（阻塞 B1-B3、主要 M4-M6、次要 m1-m6、开放问题 Q1-Q5 裁定）；[../I008-I010-joint-review.md](../I008-I010-joint-review.md)（跨迭代事项 §7）

## 1. 现状分析（证据）

### 1.1 Persona 领域与版本机制

- Persona 是独立领域对象：`scope_id` 归属 + identifier/name/tags/config/status（backend/persona/service.go:26-35），表 `personas`（backend/migrations/sql/0001_initial.sql:1401）。
- 版本机制：`persona_versions` 草稿/发布/停用（backend/persona/store.go:29-35、211-233）；`PersonaResolver.Resolve` 优先取最新 published 版本，无发布版本时回退主表 config（backend/persona/service.go:257-265）。
- Persona config 已内置角色分工：`role=manager|worker`、manager 可配置 1-10 个 `worker_persona_ids` 且不得直接持有资源工具（backend/persona/service.go:360-396）。**domain 约束的真实规则（修正原稿偏差）：`validatePersonaConfig` 仅在 config 显式声明 `role=worker` 时强制要求 `domain`（service.go:369）；config 未写 role 时默认按 worker 处理但不强制 domain。**即"无显式角色的 worker 可以无 domain"。本设计不依赖"worker 必有 domain"这一错误前提：专家组对 assistant 的域约束见 §2.1（席位校验独立于 Persona config 是否写明 domain）。
- 权限 `persona:read/manage/use`（backend/authorization/types.go:26-28）；API `/api/v1/personas` 及版本子资源（backend/httpapi/skill.go:204-231）。

### 1.2 Engine 中的专家选择与注入

- 请求级：`engine.Request.PersonaID`（backend/engine/types.go:169）；Runtime 执行前解析为 `ResolvedPersona` 并发 `persona.resolved` 事件（backend/engine/runtime.go:212-231）。
- worker 角色的 Persona 会触发 `RestrictKinds`（runtime.go:234-238）；声明 `domain` 时进一步收窄 `Context.AllowedKinds` 并过滤已解析资源上下文（runtime.go:252-265、355-361，`PersonaDomainMatches` runtime.go:381）。**注意：域收窄以 `ResolvedPersona.Role == "worker"` 为触发条件，与是否声明 domain 无关**——这是 §2.1 席位角色约束（lead 必须为 manager）的直接依据。
- Persona 指令覆盖 Skill 指令前的专家约束、工具白名单为最终限制（docs/design/ai-engine.md:133-139）。

### 1.3 诊断场景（现状：单 Persona 手工选择）

- 诊断会话/消息携带可选 `PersonaID`，由用户在 Start/AppendMessage 请求里显式传入（backend/httpapi/diagnosis.go:190、256；backend/diagnosis/types.go:53）。
- 协同调查机制：orchestrator 解析该 Persona，若是 manager 则展开其 `WorkerPersonaIDs` 为 `expert_N` 席位供 `diagnosis_investigate` 工具委派，worker 域不匹配目标资源会被拒绝（backend/diagnosis/investigation.go:178-198、248）；子任务以该专家 Persona 单独执行（investigation.go:345-374）。
- 结论：**诊断的"专家团队"= 用户每轮手工指定的一个 manager Persona 及其静态 worker 列表**，与应用/系统无关，没有按系统配备、没有应用级默认。

### 1.4 巡检场景（现状：策略绑定单 Persona）

- 巡检策略 `inspection_policies.persona_id` 为单个可选 Persona（backend/inspection/types.go:33；store.go:65；service.go:131-140 校验其为合法 Persona 资源），未配置时用内置巡检契约（docs/design/ai-engine.md:117）。无专家组概念。

### 1.5 Application 与 Resource 的关联模型

- Application 的 Instance 通过 `TargetResourceID` 指向宿主运行时资源，Dependency 通过 `TargetResourceID` + `dependency_kind` + `binding` 指向依赖资源（backend/application/types.go:41、54-58）。
- 诊断执行时由 ApplicationContext 生成 `ResourceIDs`、`ResourceBindings`、`LocatorHints` 注入 Engine（backend/diagnosis/orchestrator.go:417；backend/diagnosis/types.go:133-142 DiagnosticContext 快照）。**资源上下文链路已完备，专家组只需复用，不需复制。**

### 1.6 场景标签与 Scope 继承参照

- `engine_scope_bindings` 场景标签固定 `general/diagnosis/inspection/workflow`，scope 内同标签唯一默认绑定，子 scope 沿 scopes 父链继承（`WITH RECURSIVE chain ... row_number() OVER (PARTITION BY tag ORDER BY depth)`，backend/engine/catalog.go:85-101；表结构 backend/migrations/sql/0001_initial.sql:953-961，无主键约束、tag 有 CHECK）。**llm-domains.md §41 约定：无场景专用绑定时由 general 绑定兜底——专家组绑定沿用同一兜底语义（见 §2.2）。**

### 1.7 现状缺口小结

1. 专家选择是"每次执行手工指定单 Persona"，没有按系统/应用的专属团队配置；
2. 团队编队（manager+workers）藏在 Persona config 里，不可独立版本管理、不可按应用复用与绑定；
3. 巡检只能绑定单 Persona，无法多专家协同。

## 2. 需求分析

### 2.1 领域概念定义

**ExpertGroup（专家组）**：面向某类系统/应用的可复用专家团队配置，独立于 Persona 存在。成员是「席位」而非 Persona 本体：每个成员引用一个 Persona 并声明其在组内的分工。

**成员属性**：
- `persona_id`：引用 Persona；执行时解析其最新 published 版本（沿用 Persona 版本机制，**不锁版本**，已裁定见 §4 Q1）。
- `seat`（席位），角色约束为**硬性校验（管理端保存 + 执行期解析双重校验）**：
  - `lead`（主诊）：**仅允许 `role=manager` 的 Persona**。理由：runtime.go:234-238 的域收窄以 `Role == "worker"` 为触发条件，若 lead 为 worker（无论是否声明 domain），整个会话的 `AllowedKinds`/资源上下文都会被收窄，与 lead"总览全部关联资源"的语义直接冲突。
  - `assistant`（辅诊）：**仅允许 `role=worker` 的 Persona**。理由：禁止 assistant 为 manager，杜绝"lead → assistant(manager) → assistant 的 worker"两层展开，保住"复用既有单层委派、不引入新调度器"的边界（对齐评审 §2.2/B2）。assistant 是否声明 `domain` 不作强制（与 Persona 域校验真实规则一致，见 §1.1），声明了则按现状经 `PersonaDomainMatches` 过滤资源。
  - `oncall`（值班）：首版仅预留枚举值（DB CHECK 含 oncall），不实现时间段路由，API 保存 oncall 时拒绝（已裁定见 §4 Q4）。
- `scene_tags`：生效场景（diagnosis/inspection/workflow），空表示全场景（含 general）。
- `priority`：同席位同场景多专家时的选择顺序（数值小者优先）。

**协作策略**：首版不引入新运行时。lead 成员即现状的 manager Persona；assistant 成员直接作为 worker 席位进入既有单层 manager→worker 委派与域过滤（investigation.go 机制）。**lead Persona 自带的 `WorkerPersonaIDs` 在专家组解析时被显式忽略，以组成员为准**（已裁定见 §4 Q2），并在解析时记录一条 `expert_group.resolved` 事件的告警属性（`ignored_worker_persona_ids`），管理端在 lead 为 manager 且带 worker 列表时给出显著提示。

**会话冻结**：诊断会话在首条消息解析专家组后，将"组 ID + 成员快照（persona_id、seat、scene_tags、解析出的 persona version）"冻结到会话记录；后续 AppendMessage 轮次复用该快照，会话中途编辑组成员不影响进行中的会话（对齐评审 §3/M4）。

### 2.2 绑定关系与解析规则（含 B1 修复）

- 绑定目标：`application_id`（应用级，精确）与 `scope_id + scene_tag`（项目/团队/平台级默认）。
- 绑定表 tag 约束与 `engine_scope_bindings` 一致（general/diagnosis/inspection/workflow 四值 CHECK）。

**完整解析优先级（标签优先于层级，general 全程兜底）：**

1. 应用级绑定，`scene_tag` 精确匹配（如 diagnosis）；
2. 应用级绑定，`scene_tag=general`；
3. scope 链绑定（应用所属项目 → 团队 → 平台，深度最近者优先），`scene_tag` 精确匹配；
4. scope 链绑定，`scene_tag=general`；
5. 均无 → 未配置行为（沿用现状：单 Persona 或内置契约）。

解析伪代码：

```
resolve(application, scene):
  app = application.ProjectScopeID 所在链          # 应用不存在则跳过 1-2
  for scope_path in [exact_tag, general]:           # 标签优先
    if binding = appBinding(application.id, scope_path):
      if 复核通过(binding): return binding          # 见下方孤儿复核
  for tag in [scene, general]:
    if binding = scopeChainBinding(app.projectScope, tag):  # WITH RECURSIVE, depth 最近者优先
      return binding
  return nil                                        # 未配置，走现状
```

同层冲突确定性规则：应用级同 tag 由主键保证唯一；scope 链同 tag 多条命中时按 scope 深度最近者唯一（row_number=1），深度相同不可能（scope 树内唯一）。

**解析真值表（示例）：**

| 应用级(diagnosis) | 应用级(general) | 项目(diagnosis) | 平台(general) | 请求 scene=diagnosis 解析结果 |
|---|---|---|---|---|
| ✓ | – | – | – | 应用级(diagnosis) |
| – | ✓ | ✓ | – | 应用级(general)（应用级整层优先于 scope 链） |
| – | – | ✓ | ✓ | 项目(diagnosis)（精确标签优先） |
| – | – | – | ✓ | 平台(general) |
| – | – | – | – | 未配置，走现状 |

成员 `scene_tags` 与绑定 tag 的组合：成员 `scene_tags` 只影响"该成员在哪个执行场景参与"，不影响绑定选择。如 general 绑定 + 成员仅标 diagnosis：该组在 general 场景解析成功但该成员不参与（组内按 seat 兜底要求仍需 lead 可用，否则执行失败，见 §3.4.4）。

- 应用级绑定的 scope 校验：应用所属项目的 `ProjectScopeID`（backend/application/types.go:11）冗余存入绑定的 `scope_id`，写入时校验其在授权 scope 链上；**解析时复核应用当前 ProjectScopeID 仍与绑定 scope_id 匹配，不匹配（应用换项目后孤儿绑定）则跳过该应用级绑定、降级走 scope 链，并记录告警事件**（采纳评审 m4）。

### 2.3 关联资源作为专家上下文

专家组不持有资源。诊断/巡检执行时按现状从 ApplicationContext / 巡检目标解析 ResourceIDs 与绑定（orchestrator.go:417），专家组解析只决定"哪些 Persona 参与及各自 domain 约束"；assistant 声明的 `domain` 继续通过 `PersonaDomainMatches` 过滤资源（runtime.go:252-265），保证专家只见其领域内资源。lead 为 manager 角色不触发域收窄，其宽域视角仅用于汇总委派结果，不会把宽域泄漏到 assistant 的子任务执行（子任务以各 worker Persona 单独解析执行，investigation.go:345-374 现状不变）。

### 2.4 与既有对象的关系与边界

| 对象 | 边界 |
|---|---|
| Persona | 专家本体与契约（指令/工具白名单/Schema/版本）；ExpertGroup 只引用，不复制 |
| Skill | 执行计划与正文契约；二者可组合（Skill + ExpertGroup 同请求） |
| Resource | 资源目录与凭据；专家组不引用具体资源，资源经 Application 关联进入上下文 |
| 场景标签 | 复用 general/diagnosis/inspection/workflow 四值，ExpertGroup 绑定表同样受 tag CHECK 约束 |
| manager Persona 的 WorkerPersonaIDs | 保留兼容（单 Persona 路径不受影响）；专家组路径以成员列表为准，lead 自带列表显式忽略并告警（已裁定 Q2，见 §2.1） |

## 3. 方案设计

### 3.1 数据模型（新增迁移，编号 0005）

**迁移编号预分配：`backend/migrations/sql/0005_expert_groups.sql`**（依据联合评审 §5.1：I008 占 0004、I009 占 0005、I010 占 0006；当前非归档迁移最大编号 0003，archive/ 目录不动。若实际合入顺序变化，以后合入者重排号）。

```sql
CREATE TABLE expert_groups (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id text NOT NULL DEFAULT 'default',
    scope_id uuid NOT NULL,
    name text NOT NULL,
    identifier text NOT NULL,
    description text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active',  -- active | disabled
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE (scope_id, identifier)
);

CREATE TABLE expert_group_members (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id uuid NOT NULL REFERENCES expert_groups(id),
    persona_id uuid NOT NULL REFERENCES personas(id),
    seat text NOT NULL DEFAULT 'assistant',   -- lead | assistant | oncall（CHECK 约束）
    scene_tags jsonb NOT NULL DEFAULT '[]',   -- 子集 of diagnosis/inspection/workflow，空=全部
    priority integer NOT NULL DEFAULT 100,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (group_id, persona_id)
);
-- lead 唯一：partial unique index
CREATE UNIQUE INDEX expert_group_members_single_lead
    ON expert_group_members (group_id) WHERE seat = 'lead';

CREATE TABLE expert_group_bindings (
    scope_id uuid NOT NULL,
    scene_tag text NOT NULL CHECK (scene_tag IN ('general','diagnosis','inspection','workflow')),
    group_id uuid NOT NULL REFERENCES expert_groups(id),
    application_id uuid,                   -- NULL = scope 级默认；非 NULL = 应用级（scope_id 冗余存应用当前 ProjectScopeID）
    created_by uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (scope_id, scene_tag, COALESCE(application_id, '00000000-0000-0000-0000-000000000000'::uuid))
);
```

**实现要求与取舍说明：**
- 每个组**必须且仅 1 个 lead**、成员数上限 20：保存时应用层校验 + lead 唯一性由 partial unique index 兜底（采纳 m3）；空组 / 无 lead 组在保存时直接拒绝（采纳 M5-3）。
- `expert_group_members` 采用 PUT 整体替换（物理删除旧成员行），不加 deleted_at；理由：成员行无独立生命周期与引用关系，整体替换语义下软删只增加噪音；补 `updated_at` 以留审计痕迹（采纳 m2，部分采纳）。
- `expert_group_bindings` 保留表达式主键（应用级与 scope 级同表需唯一性保证），并补齐 tag CHECK 对齐既有表（采纳 m1）。
- 成员 Persona 保存期即做授权 scope 可见性校验（前移，避免错误延迟到执行期；采纳 m5），执行期解析时再复核一次（Persona 删除/软删/停用、跨 scope 均在两处拦截）。
- scope 链继承查询（`WITH RECURSIVE ... row_number()`）**抽公共 helper** 供 engine_scope_bindings 与 expert_group_bindings 共用，作为 T01 验收项（采纳联合评审 §7-6）。

不新增对象级授权表，权限随 Scope RBAC 继承（对齐 docs/design/llm-domains.md 权限模型）。

### 3.2 权限

`expert_group:read` / `expert_group:manage` / `expert_group:use`（authorization/types.go 增加，模式同 persona，backend/authorization/types.go:26-28）。

### 3.3 API（对齐 /api/v1/personas 风格，backend/httpapi/skill.go:204-231）

- `GET/POST /api/v1/expert-groups`（read/manage）
- `PATCH /api/v1/expert-groups/{groupID}`（manage；名称/描述/状态）
- `GET/PUT /api/v1/expert-groups/{groupID}/members`（manage；整体替换成员，含席位角色/lead 唯一/scope 可见性校验）
- `GET/PUT/DELETE /api/v1/scopes/{scopeID}/expert-group-bindings?scene_tag=&application_id=`（manage）
- `GET /api/v1/applications/{applicationID}/effective-expert-group?scene_tag=`（read；按 §2.2 完整优先级解析并返回生效组及成员，便于前端预览）

新包 `backend/expertgroup`（service.go/store.go/types.go），由 httpapi 注册路由，模式同 backend/persona。

### 3.4 解析职责分层与 Engine 注入流程（含 B3 修复）

**职责分层（对齐 docs/design/ai-engine.md §9：engine 不依赖 Skill/Diagnosis/ExpertGroup 等业务域）：**

1. **应用级绑定解析在 diagnosis/inspection 调用方完成**：诊断 orchestrator / 巡检 executor 在构造 `engine.Request` 前解析生效专家组（含应用级 + scope 链全优先级），把结果折算进请求；Runtime 内不出现 Application 概念。
2. **Runtime 不查库**：engine 包定义 `ExpertGroupResolver` 注入接口（模式同既有 `personaResolver`），仅负责"组 ID + 场景 → 成员席位集合"的映射契约；具体实现由 backend/expertgroup 提供、在组装层注入。请求已带解析好的 `ExpertGroupID` 时，Runtime 通过该接口取成员、发 `expert_group.resolved` 事件（对齐 `persona.resolved`，runtime.go:231）。事件类型在 I008 冻结契约文档中以"平台生命周期事件"扩展类目登记（联合评审 §7-2）。
3. `engine.Request` 增加 `ExpertGroupID string`（`omitempty`，types.go:169 附近）；若请求同时带显式 `PersonaID`，**以显式 PersonaID 为准、忽略 ExpertGroupID 并告警**（请求级优先，与会话级规则见 §3.4.6）。

**诊断：**
4. `StartInput`/`AppendMessageInput` 增加 `ExpertGroupID`；未显式传入时按 §2.2 优先级解析默认组。`investigationTools` 改造：专家来源由"单 manager 的 WorkerPersonaIDs"扩展为"lead Persona + 组内 assistant 成员"（investigation.go:178-198），域匹配校验（investigation.go:248）不变。**assistant 一律为 worker 角色（§2.1），不再二次展开任何内嵌 worker 列表。**
5. 会话首条消息解析后冻结成员快照到会话记录（§2.1 会话冻结）；后续轮次复用快照。会话已带 ExpertGroupID 时，AppendMessage 显式传入的 PersonaID 优先于会话冻结组（单轮切换专家，不改变会话冻结组本身）。

**巡检：**
6. `inspection_policies` 增加 `expert_group_id` 列（可空）；**与 `persona_id` 互斥，创建/更新策略时二者同时设置直接校验拒绝**（已裁定 Q3，不采用静默优先）。执行时同上解析；巡检 run 记录持久化"组 ID + 各成员 persona_id + 解析出的 version"快照（§3.4.8）。

**失败语义与边界场景（采纳 M5）：**
7. 组或必需成员解析失败（Persona 删除/软删/停用、跨 scope 不可见、lead 缺失）按现状 invalid 错误返回，**不静默降级；lead 失败即整体失败**（不存在"lead 失败但 assistant 可用"的部分执行）。
8. **组停用（disabled）等价于无绑定**：解析到停用组时跳过该绑定、按 §2.2 优先级继续向下解析（不报错），并在事件流记录告警。配置期对"绑定指向停用组"给出显著提示。
9. 审计与可复现（采纳 M4）：`expert_group.resolved` 事件、诊断会话冻结快照、巡检 run 记录三处均持久化"组 + 成员 + 各成员解析版本"快照，满足事后追溯（Q1 裁定的配套要求）。
10. 并发：成员 PUT 整体替换与进行中执行并发——执行开始前取快照即可，不锁配置；同 scope+tag+application 绑定写入竞争由主键唯一性保证。

### 3.5 前端

"专家"页新增专家组 Tab：组列表、成员编辑（选择 Persona + 席位 + 场景，管理端强制席位角色约束并在 lead 带 worker 列表时提示忽略）、绑定管理（scope 场景默认 + 应用级）；应用详情/诊断会话/巡检策略表单增加"专家组"选择器，默认展示 effective-expert-group 解析结果。诊断会话表单与 I008 的 Engine 选择器做 UI 排版协调（联合评审 §2）。

### 3.6 迁移与灰度

- 纯新增表 + 可空列，无数据回填；老请求不带 `expert_group_id` 行为完全不变。
- 灰度顺序：T01-T03（目录+API）→ T04（Engine，feature 无行为变化直至有绑定）→ T05（诊断/巡检接入）→ T06（前端）。每步可独立回归。
- **跨迭代硬前置：T04（Request 加字段 + `expert_group.resolved` 事件）必须在 I008-M1 契约冻结基线合入 main 之后实施**，并补录含 ExpertGroupID 的契约用例（联合评审 §7-1/7-2）；T01-T03 无此依赖，可先行。

## 4. 开放问题裁定记录（2026-11-18，全部已裁定关闭）

1. **Q1 成员是否锁定 Persona 版本？——已裁定：不锁版本，跟随最新 published。** 与单 Persona 行为一致（service.go:257-265）；可复现性以执行审计持久化"组 + 成员 + 解析版本"快照保障（§3.4.9）；`pinned_version_id` 列入 backlog。
2. **Q2 与 manager Persona `worker_persona_ids` 的冲突——已裁定：以组成员为准，解析时显式忽略 lead 自带 worker 列表并记录告警事件（`ignored_worker_persona_ids`）；管理端保存时显著提示。** 同时收紧 assistant 仅允许 worker 角色（§2.1）。
3. **Q3 巡检互斥策略——已裁定：`persona_id` 与 `expert_group_id` 同时设置时校验直接拒绝**（互斥，不静默优先；巡检为无人值守场景，静默语义难排查）。
4. **Q4 值班（oncall）席位——已裁定：首版仅预留枚举值**（DB CHECK 含 oncall），不实现时间段路由；API 保存 oncall 成员时拒绝。时间段路由列入 backlog。
5. **Q5 平台级内置专家组——已裁定：首版不随发布包安装内置专家组。** 专家团队强依赖各组织的资源领域与 Persona 资产，平台预置组大概率引用不存在的 Persona；待运营数据沉淀后再评估。

## 5. 验收标准（可测条目，采纳 m6）

1. 绑定解析四级优先级 + 标签 general 兜底：按 §2.2 真值表逐行用例覆盖；
2. 席位角色约束：lead=manager、assistant=worker，保存期与执行期双重校验用例；
3. lead 自带 WorkerPersonaIDs 被忽略且产生告警事件；
4. 成员域过滤行为：assistant 声明 domain 时资源按 `PersonaDomainMatches` 过滤，lead 不收窄；
5. 无专家组的既有执行路径回归不变（单 Persona / 内置契约）；
6. 专家组停用等价于无绑定、按优先级降级解析；
7. 巡检 persona_id + expert_group_id 并存被校验拒绝；
8. 诊断会话成员快照冻结：会话中编辑组不影响进行中会话；
9. 审计三处（事件/会话/巡检 run）含"组 + 成员 + 解析版本"快照；
10. 边界场景：成员 Persona 删除/停用、跨 scope 引用、空组/无 lead 保存拒绝、应用换项目后孤儿绑定降级。

## 6. 评审次要问题采纳记录

| # | 问题 | 处置 | 理由 |
|---|---|---|---|
| m1 | 表达式主键非项目惯例、缺 tag CHECK | **采纳（部分）**：补 tag CHECK 与 lead 唯一 partial index；表达式主键保留（应用级与 scope 级同表需要唯一性保证），在 §3.1 注明 | 唯一性是正确性需求，CHECK 是风格对齐 |
| m2 | members 无 updated_at/软删 | **采纳（部分）**：补 updated_at；不引入软删并注明理由 | 整体替换语义下软删无意义 |
| m3 | lead 唯一/成员上限的 DB 层要求 | **采纳**：partial unique index + 应用层校验 | 双保险 |
| m4 | 应用换 scope 后孤儿绑定 | **采纳**：解析时复核并降级 + 告警（§2.2） | 避免静默失效 |
| m5 | 成员 scope 可见性校验前移 | **采纳**：保存期校验 + 执行期复核（§3.1） | 错误暴露不应延迟到半夜巡检 |
| m6 | 缺验收标准小节 | **采纳**：新增 §5 | 可测性 |
