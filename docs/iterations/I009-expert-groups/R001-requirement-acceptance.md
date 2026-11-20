# R001 验收记录：专家组领域与执行链

**所属迭代：** [I009](iteration.md) | **验收日期：** 2026-11-18 | **状态：** 实施完成（待运行环境验收）

## 交付范围与证据

| 项 | 交付 | 证据 |
| --- | --- | --- |
| 领域与迁移 0005 | expert_groups / expert_group_members（lead 唯一部分索引、seat CHECK、≤20 应用层校验）/ expert_group_bindings（tag CHECK+表达式主键）；inspection_policies.expert_group_id；diagnosis_sessions 快照列；inspection_runs 快照列；role_permissions 派生 expert_group:* 授权 | commit 8cc1ab1 `backend/migrations/sql/0005_expert_groups.sql(+down)` |
| 席位与校验（B2 裁定） | lead=manager、assistant=worker 双重校验；oncall 拒绝；空组/无 lead 拒绝 | 8cc1ab1 `backend/expertgroup/service.go` |
| 四级绑定解析（B1 裁定） | 应用级精确→应用级 general→scope 链精确→scope 链 general（标签优先于层级）；孤儿绑定/停用组降级+告警 | `backend/expertgroup/resolver.go`，真值表全覆盖测试 |
| 执行链注入（B3 裁定） | engine.Request 增 expert_group_id（契约白名单同步登记）；ExpertGroupResolver 接口由调用方注入，Runtime 不查库 | `backend/engine/expert_group.go`、adapter_contract_test.go 白名单更新 |
| Q1 可复现快照 | 诊断 Start 冻结「组+成员+各成员解析版本」入会话列并发 `expert_group.resolved` 事件；巡检 run 持久化快照 | `backend/diagnosis/`、`backend/inspection/` |
| Q2/Q3 裁定 | 专家改由快照 assistant 供给、忽略 lead 内嵌 worker 并在事件记 ignored_worker_persona_ids；巡检单 Persona 与组互斥直接拒绝 | investigation.go 改造、inspection 校验 |
| API 与装配 | /api/v1/expert-groups CRUD+members、/scopes/{id}/expert-group-bindings、/applications/{id}/effective-expert-group；cmd/api 与 cmd/worker 装配 | `backend/httpapi/` |
| 前端 | 专家页新增「专家组」Tab（组列表/席位编辑/绑定管理） | `ExpertGroupsPanel.svelte`、`api.ts` |

## 验证结果

- `go build ./...`、`go vet` 通过；`go test ./expertgroup/... ./engine/... ./httpapi/... ./diagnosis/... ./inspection/... ./migrations/... -count=1`：414 passed。
- 前端 `npm run check` 0 errors、`npm run build` ✓、`npm test` 105 passed（专家组面板 DropdownSelect 事件用法类型错误已在 f0adedd 修复）。
- migrations_test 迁移计数 5→6 由 C1 迭代统一更新。

## 未尽事项

1. scope 链查询未抽公共 helper（engine/catalog.go 未动，SQL 复用同模式）——已记录偏差，待后续重构。
2. 集成测试未覆盖 PostgreSQL 真库路径（逻辑层由 fake store 单测覆盖），待运行环境验收。
