# I009 专家组能力

**迭代编号：** I009
**目录标识：** I009-expert-groups
**状态：** 设计评审中
**计划开始：** 2026-11-18
**计划封板：** 待定

## 1. 迭代目标

把"专家"从单次执行手工选择的一个 Persona，升级为可按系统/应用配备的「专家组（ExpertGroup）」：每个应用（或项目/团队/平台）可以配置一组具备角色分工的专家，专家组与该应用既有的关联资源（Instance/Dependency 所引用的 Resource）绑定，诊断、巡检等场景在执行时由专属专家组基于关联资源上下文协同运维。

本迭代不新增资源类型、不复制资源数据：应用与资源的关联继续复用 Application 的 Instance/Dependency/Relation 模型，专家组只引用 Persona 与应用绑定关系。

## 2. 已确认的设计决策

1. ExpertGroup 是独立一级领域对象，归属 `scope_id`，管理入口位于"专家"页面扩展；不进入通用资源目录（对齐 Provider/Engine/Skill/Persona 的独立领域模式，见 docs/design/llm-domains.md）。
2. ExpertGroup 成员引用 Persona（执行时解析其最新已发布版本），每个成员声明 `seat`（席位：lead 主诊 / assistant 辅诊 / oncall 值班）与生效场景标签（diagnosis/inspection/workflow），不在 ExpertGroup 内复制 Persona 指令或工具白名单。
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
| R001 | 专家组领域与应用绑定 | T01-T06 | 设计评审中 | [R001-requirement.md](R001-requirement.md) | 待补充 |

## 5. 任务和依赖

| 任务 | 名称 | 依赖 | 主要产出 | 状态 |
|---|---|---|---|---|
| T01 | ExpertGroup 数据模型与迁移 | 无 | expert_groups / expert_group_members / expert_group_bindings 表 | 待批准 |
| T02 | ExpertGroup 领域服务与权限 | T01 | backend/expertgroup 服务、scope 校验、expert_group:* 权限 | 待批准 |
| T03 | API 与路由 | T02 | /api/v1/expert-groups 及绑定接口 | 待批准 |
| T04 | Engine 专家组解析与注入 | T02 | Runtime 解析 expert_group_id，成员进入现有委派链 | 待批准 |
| T05 | 诊断/巡检调用链接入 | T04 | 诊断 StartInput、巡检策略支持 expert_group_id | 待批准 |
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

## 8. 封板记录

待补充。
