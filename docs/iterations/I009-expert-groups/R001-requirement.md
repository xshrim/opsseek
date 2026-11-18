# R001 专家组领域与应用绑定（需求分析与方案设计）

## 1. 现状分析（证据）

### 1.1 Persona 领域与版本机制

- Persona 是独立领域对象：`scope_id` 归属 + identifier/name/tags/config/status（backend/persona/service.go:26-35），表 `personas`（backend/migrations/sql/0001_initial.sql:1401）。
- 版本机制：`persona_versions` 草稿/发布/停用（backend/persona/store.go:29-35、211-233）；`PersonaResolver.Resolve` 优先取最新 published 版本，无发布版本时回退主表 config（backend/persona/service.go:257-265）。
- Persona config 已内置角色分工：`role=manager|worker`、worker 必须有 `domain`（资源领域）、manager 可配置 1-10 个 `worker_persona_ids` 且不得直接持有资源工具（backend/persona/service.go:360-396）。即"一个 manager 带若干 worker"的静态编队已经存在，但它是 Persona 内部字段，不是可独立管理、可绑定到系统的对象。
- 权限 `persona:read/manage/use`（backend/authorization/types.go:26-28）；API `/api/v1/personas` 及版本子资源（backend/httpapi/skill.go:204-231）。

### 1.2 Engine 中的专家选择与注入

- 请求级：`engine.Request.PersonaID`（backend/engine/types.go:169）；Runtime 执行前解析为 `ResolvedPersona` 并发 `persona.resolved` 事件（backend/engine/runtime.go:212-231）。
- worker Persona 的 `domain` 会收窄 `Context.AllowedKinds`（backend/engine/runtime.go:234-237）并过滤已解析资源上下文（runtime.go:252-253、355-361，`PersonaDomainMatches` runtime.go:381）。
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

- `engine_scope_bindings` 场景标签固定 `general/diagnosis/inspection/workflow`，scope 内同标签唯一默认绑定，子 scope 沿 scopes 父链继承（`WITH RECURSIVE chain ... row_number() OVER (PARTITION BY tag ORDER BY depth)`，backend/engine/catalog.go:85-101；表结构 backend/migrations/sql/0001_initial.sql:953-961）。

### 1.7 现状缺口小结

1. 专家选择是"每次执行手工指定单 Persona"，没有按系统/应用的专属团队配置；
2. 团队编队（manager+workers）藏在 Persona config 里，不可独立版本管理、不可按应用复用与绑定；
3. 巡检只能绑定单 Persona，无法多专家协同。

## 2. 需求分析

### 2.1 领域概念定义

**ExpertGroup（专家组）**：面向某类系统/应用的可复用专家团队配置，独立于 Persona 存在。成员是「席位」而非 Persona 本体：每个成员引用一个 Persona 并声明其在组内的分工。

**成员属性**：
- `persona_id`：引用 Persona；执行时解析其最新 published 版本（沿用 Persona 版本机制，默认不锁版本，见开放问题 Q3）。
- `seat`（席位）：`lead`（主诊，必须是 manager 角色或无域约束专家，负责汇总）/ `assistant`（辅诊）/ `oncall`（值班，低频参与）。首版仅 lead + assistant，oncall 预留。
- `scene_tags`：生效场景（diagnosis/inspection/workflow），空表示全场景。
- `priority`：同席位同场景多专家时的选择顺序。

**协作策略**：首版不引入新运行时。lead 成员即现状的 manager Persona，assistant 成员展开为其 `WorkerPersonaIDs`（或本身为 worker），进入既有单层 manager→worker 委派与域过滤（investigation.go 机制）。专家组是"配置层编队"，不是新调度器。

### 2.2 绑定关系

- 绑定目标：`application_id`（应用级，精确）与 `scope_id + scene_tag`（项目/团队/平台级默认）。
- 解析优先级：**应用级绑定 → 项目 scope → 团队 scope → 平台 scope 兜底**（同一 scope 树内按深度最近者优先，复用 engine_scope_bindings 的继承语义）。
- 应用级绑定的 scope 校验：应用所属项目的 `ProjectScopeID`（backend/application/types.go:11）必须在当前授权 scope 链上。

### 2.3 关联资源作为专家上下文

专家组不持有资源。诊断/巡检执行时按现状从 ApplicationContext / 巡检目标解析 ResourceIDs 与绑定（orchestrator.go:417），专家组解析只决定"哪些 Persona 参与及各自 domain 约束"；worker 的 `domain` 继续通过 `PersonaDomainMatches` 过滤资源（runtime.go:252-261），保证专家只见其领域内资源。

### 2.4 与既有对象的关系与边界

| 对象 | 边界 |
|---|---|
| Persona | 专家本体与契约（指令/工具白名单/Schema/版本）；ExpertGroup 只引用，不复制 |
| Skill | 执行计划与正文契约；二者可组合（Skill + ExpertGroup 同请求） |
| Resource | 资源目录与凭据；专家组不引用具体资源，资源经 Application 关联进入上下文 |
| 场景标签 | 复用 general/diagnosis/inspection/workflow 四值，ExpertGroup 绑定表同样受 tag 约束 |
| manager Persona 的 WorkerPersonaIDs | 保留兼容；专家组绑定时以成员列表为准展开（见开放问题 Q2） |

## 3. 方案设计

### 3.1 数据模型（新增迁移）

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
    seat text NOT NULL DEFAULT 'assistant',   -- lead | assistant | oncall
    scene_tags jsonb NOT NULL DEFAULT '[]',   -- 子集 of diagnosis/inspection/workflow，空=全部
    priority integer NOT NULL DEFAULT 100,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (group_id, persona_id)
);

CREATE TABLE expert_group_bindings (
    scope_id uuid NOT NULL,
    scene_tag text NOT NULL,               -- general|diagnosis|inspection|workflow
    group_id uuid NOT NULL REFERENCES expert_groups(id),
    application_id uuid,                   -- NULL = scope 级默认；非 NULL = 应用级
    created_by uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (scope_id, scene_tag, COALESCE(application_id, '00000000-...'::uuid))
);
```

约束：每个组最多 1 个 lead；成员数上限 20；成员 Persona 必须与本组同 scope 链可见（解析时校验，不加外键级联）。不新增对象级授权表，权限随 Scope RBAC 继承（对齐 docs/design/llm-domains.md 权限模型）。

### 3.2 权限

`expert_group:read` / `expert_group:manage` / `expert_group:use`（authorization/types.go 增加，模式同 persona，backend/authorization/types.go:26-28）。

### 3.3 API（对齐 /api/v1/personas 风格，backend/httpapi/skill.go:204-231）

- `GET/POST /api/v1/expert-groups`（read/manage）
- `PATCH /api/v1/expert-groups/{groupID}`（manage；名称/描述/状态）
- `GET/PUT /api/v1/expert-groups/{groupID}/members`（manage；整体替换成员）
- `GET/PUT/DELETE /api/v1/scopes/{scopeID}/expert-group-bindings?scene_tag=&application_id=`（manage）
- `GET /api/v1/applications/{applicationID}/effective-expert-group?scene_tag=`（read；返回按继承优先级解析出的生效组及成员，便于前端预览）

新包 `backend/expertgroup`（service.go/store.go/types.go），由 httpapi 注册路由，模式同 backend/persona。

### 3.4 Engine 解析与注入流程

1. `engine.Request` 增加 `ExpertGroupID string`（types.go:169 附近）；Runtime 在 Persona 解析前增加专家组解析步骤（runtime.go:212 前插入）：若请求带 `ExpertGroupID` 且无显式 `PersonaID`，解析组内该场景生效成员，取 lead 的 PersonaID 作为请求 PersonaID，assistant 集合作为展开的 worker 席位来源；发 `expert_group.resolved` 事件（对齐 `persona.resolved`，runtime.go:231）。
2. 诊断：`StartInput`/`AppendMessageInput` 增加 `ExpertGroupID`；未显式传入时按应用级→scope 继承解析默认组。`investigationTools` 改造：专家来源由"单 manager 的 WorkerPersonaIDs"扩展为"lead Persona + 组内 assistant 成员"（investigation.go:178-198），域匹配校验（investigation.go:248）不变。
3. 巡检：`inspection_policies` 增加 `expert_group_id` 列（可空，与 persona_id 互斥或 persona 优先），执行时同上解析。
4. 组或成员解析失败（Persona 停用、域缺失）按现状 invalid 错误返回，不静默降级。

### 3.5 前端

"专家"页新增专家组 Tab：组列表、成员编辑（选择 Persona + 席位 + 场景）、绑定管理（scope 场景默认 + 应用级）；应用详情/诊断会话/巡检策略表单增加"专家组"选择器，默认展示 effective-expert-group 解析结果。

### 3.6 迁移与灰度

- 纯新增表 + 可空列，无数据回填；老请求不带 `expert_group_id` 行为完全不变。
- 灰度顺序：T01-T03（目录+API）→ T04（Engine，feature 无行为变化直至有绑定）→ T05（诊断/巡检接入）→ T06（前端）。每步可独立回归。

## 4. 开放问题

1. **Q1 成员是否锁定 Persona 版本？** 当前设计"跟随最新发布版本"（与单 Persona 行为一致）；巡检可复现性要求高，是否允许成员声明 `pinned_version_id`？
2. **Q2 与 manager Persona `worker_persona_ids` 的冲突**：组内 lead 是 manager 时，其自带 worker 列表与组内 assistant 成员取并集还是以组为准？当前设计：以组成员为准展开，lead 自带列表忽略（需在文档与校验中明确）。
3. **Q3 巡检互斥策略**：`persona_id` 与 `expert_group_id` 同时设置时以谁为准，还是直接校验拒绝？
4. **Q4 值班（oncall）席位**：首版是否实现（按时间段路由），还是仅预留枚举值？
5. **Q5 平台级内置专家组**：是否随发布包安装平台级默认专家组（类似内置 Skill 的安装模式，见 I005）？
