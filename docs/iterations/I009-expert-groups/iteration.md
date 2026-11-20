# I009 专家组能力

**迭代编号：** I009
**目录标识：** I009-expert-groups
**状态：** 实施完成（待运行环境验收）
**计划开始：** 2026-11-18
**计划封板：** 待定

## 1. 迭代目标

把"专家"从单次执行手工选择的一个 Persona，升级为可按系统/应用配备的「专家组（ExpertGroup）」：每个应用（或项目/团队/平台）可以配置一组具备角色分工的专家，专家组与该应用既有的关联资源（Instance/Dependency 所引用的 Resource）绑定，诊断、巡检等场景在执行时由专属专家组基于关联资源上下文协同运维。

本迭代不新增资源类型、不复制资源数据：应用与资源的关联继续复用 Application 的 Instance/Dependency/Relation 模型，专家组只引用 Persona 与应用绑定关系。

## 2. 已确认的设计决策

1. ExpertGroup 是独立一级领域对象，归属 `scope_id`，管理入口位于"专家"页面扩展；不进入通用资源目录（对齐 Provider/Engine/Skill/Persona 的独立领域模式，见 docs/design/llm-domains.md）。
2. ExpertGroup 成员引用 Persona（执行时解析其最新已发布版本，不锁版本），每个成员声明 `seat`（席位：lead 主诊，仅允许 manager 角色 / assistant 辅诊，仅允许 worker 角色 / oncall 值班，仅预留枚举）与生效场景标签（diagnosis/inspection/workflow），不在 ExpertGroup 内复制 Persona 指令或工具白名单；lead 自带 `WorkerPersonaIDs` 在专家组路径被显式忽略并告警。
3. ExpertGroup 通过绑定表挂到应用（可选）与 scope+场景标签，继承顺序：应用级 → 项目 → 团队 → 平台兜底，复用 `engine_scope_bindings` 的 Scope 继承查询模式。
4. 权限使用独立前缀 `expert_group:read` / `expert_group:manage` / `expert_group:use`，沿用 Scope RBAC 继承，不新增对象级授权表。
5. 未配置专家组的场景沿用现状：单 Persona（manager/worker）或内置契约，不强制迁移。

## 3. 交付边界

- ExpertGroup 目录：创建、更新、成员管理、启用停用；
- 绑定管理：scope/场景/应用级默认专家组绑定；
- Engine 执行链：`expert_group_id` 请求字段、专家组解析、成员注入（复用现有 manager→worker 单层委派与域过滤）；
- 诊断会话与巡检策略支持选择专家组；
- 前端"专家"页面扩展：专家组管理、应用绑定配置。

非目标：多级专家委派图、运行时动态组队、在 ExpertGroup 内自定义工具白名单、复制 Resource 数据、修改 Persona 版本机制。

## 4. 需求清单

| 需求 | 名称 | 任务 | 状态 | 需求文档 | 验收报告 |
|---|---|---|---|---|---|
| R001 | 专家组领域与应用绑定 | T01-T06 | 实施完成 | [R001-requirement.md](R001-requirement.md) | [R001-requirement-acceptance.md](R001-requirement-acceptance.md) |

## 5. 任务和依赖

| 任务 | 名称 | 依赖 | 主要产出 | 状态 |
|---|---|---|---|---|
| T01 | ExpertGroup 数据模型与迁移 | 无 | 迁移 0005：expert_groups / expert_group_members / expert_group_bindings 表 | 待批准 |
| T02 | ExpertGroup 领域服务与权限 | T01 | backend/expertgroup 服务、scope 校验、expert_group:* 权限 | 待批准 |
| T03 | API 与路由 | T02 | /api/v1/expert-groups 及绑定接口 | 待批准 |
| T04 | Engine 专家组解析与注入 | T02、I008 A2 契约基线合入 | Runtime 经注入接口（不查库）处理 expert_group_id，成员进入现有委派链 | 待批准 |
| T05 | 诊断/巡检调用链接入 | T04 | 调用方完整四级绑定解析（应用级在调用方完成）、诊断 StartInput、巡检策略支持 expert_group_id | 待批准 |
| T06 | 前端专家页扩展 | T03 | 专家组管理与应用绑定 UI | 待批准 |

## 6. 进入条件

- Persona 独立领域、版本与 manager/worker 委派机制可用（backend/persona、backend/engine/runtime.go）；
- Application 与 Resource 的 Instance/Dependency 关联、DiagnosticContext 快照链路可用（backend/application、backend/diagnosis/orchestrator.go）；
- `engine_scope_bindings` 的 scope 继承查询模式可参照复用（backend/engine/catalog.go:85）。

## 7. 退出条件

- 专家组可创建、编辑成员、绑定到应用/scope 场景并可停用；
- 诊断、巡检在配置专家组后由组内成员协同执行，行为有审计与测试；
- 应用级→项目→团队→平台的绑定继承可复核；
- 未配置专家组的既有执行路径行为不变（兼容回归）；
- 开放问题全部关闭并进入实施或 backlog。

## 8. 裁定记录（2026-11-18）

用户已批准 [R001-design-review.md](R001-design-review.md) 的推荐裁定，返修要点已合入 R001-requirement.md v2：

- **B1** 绑定解析补全：标签优先于层级、general 全程兜底、四级优先级 + 伪代码 + 真值表（R001 §2.2）。
- **B2** 席位角色硬约束：lead 仅 manager、assistant 仅 worker，双重校验；lead 宽域不泄漏到 assistant 子任务（R001 §2.1/§2.3）。
- **B3** 解析职责分层：应用级解析在 diagnosis/inspection 调用方完成；Runtime 以注入接口（ExpertGroupResolver，模式同 personaResolver）取成员，不查库（R001 §3.4，对齐 ai-engine.md §9）。
- **Q1-Q5 全部裁定关闭**：不锁版本 + 审计快照；以组成员为准并忽略/告警 lead 内嵌列表；巡检互斥直接拒绝；oncall 仅预留枚举；不装内置专家组（R001 §4）。
- **事实偏差修正**：worker domain 仅显式 role=worker 时强制（persona/service.go:369），设计不再依赖"worker 必有 domain"。
- **跨迭代协调**（依据 I008-I010-joint-review.md）：T04/T05 硬前置 I008-M1（A2 契约基线）合入 main；迁移编号预分配 0005；`expert_group.resolved` 事件以平台生命周期事件扩展类目登记。

## 9. 任务拆解（B1 实施序列）

T01-T06 中 T04/T05 依赖 I008 A2 契约基线先行合入（标注※），其余子任务可先行。子任务粒度为可独立提交的 PR：

| 子任务 | 内容 | 依赖 | 主要产出 |
|---|---|---|---|
| S1 领域对象与迁移 | 迁移 0005（三表 + 巡检列）、expertgroup 领域类型、scope 链继承公共 helper 抽取 | 无 | backend/migrations/sql/0005_expert_groups.sql、backend/expertgroup/types.go、catalog 继承查询 helper |
| S2 服务、权限与 API | expertgroup service/store（成员校验：席位角色/lead 唯一/scope 可见性/停用互斥）、expert_group:* 权限、/api/v1/expert-groups 及绑定与 effective-expert-group 接口 | S1 | backend/expertgroup/{service,store}.go、authorization、httpapi 路由 |
| S3 调用方解析与执行链接入※ | 诊断/巡检调用方完整四级绑定解析（含孤儿复核降级）、会话冻结快照、巡检互斥校验与 run 快照、engine.Request 加 ExpertGroupID + ExpertGroupResolver 注入接口 + expert_group.resolved 事件登记 | S2、I008 A2 基线合入 | backend/diagnosis、backend/inspection、backend/engine（注入接口，不查库） |
| S4 前端专家页扩展 | 专家组 Tab、成员编辑（席位角色约束与忽略提示）、绑定管理、应用详情/诊断/巡检选择器、effective 预览 | S2（S3 可并行开发，联调需 S3） | 前端专家页与应用绑定 UI |

灰度与回归顺序：S1 → S2 → S3 → S4；S4 的选择器联调依赖 S3，纯管理与绑定 UI 仅依赖 S2。

## 10. 封板记录

待补充。

