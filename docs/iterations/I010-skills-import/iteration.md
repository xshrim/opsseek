# I010 Skills 迁移与导入

**迭代编号：** I010  
**目录标识：** I010-skills-import  
**状态：** 设计评审中  
**计划开始：** 2026-11-18  
**计划封板：** 待定

## 1. 迭代目标

为 OpsSeek 提供 Skill 导入能力，并将内置技能从单一「健康诊断」扩展为按中间件域组织的可迁移技能库。具体包括三部分：

1. **导入功能**：支持导入 OpsSeek 自身导出的 Skill 文档（JSON/YAML/Markdown，`opsseek.skill/v1`）以及社区通用的 Agent Skills（SKILL.md 目录/zip 包，agentskills.io 开放标准），完成格式转换、工具白名单收敛、安全校验和版本化落库。
2. **技能迁移盘点**：对照 OpsKeeper 时期（git 历史）的技能能力，确认迁移收敛过程中产生的能力差异并补齐（重点是工具白名单与内置技能覆盖面）。
3. **技能库组织**：扩展 `backend/skill/builtin/` 为按中间件域分类的内置/社区技能库，制定首批中间件运维技能清单。

本迭代复用 I005 已交付的 Skill 独立领域基础（独立表、`scope_id` 归属、`skill:read/manage/execute` 权限、版本不可变与发布流程，见 [I005](../I005-skill-intelligent-inspection/iteration.md)），不改变 Engine、Provider、Persona 的领域边界（见 [docs/design/llm-domains.md](../../design/llm-domains.md)）。

## 2. 已确认的设计决策（评审中）

1. Skill 导入是对 `CreateVersion` + Skill 创建的组合封装，不绕过现有校验路径；导入的 Skill 版本默认为 `draft` 状态，需人工发布。
2. 外部格式（Agent Skills SKILL.md）导入时必须完成向 `opsseek.skill/v1` 文档的确定性转换；无法映射的字段（如 scripts/ 可执行代码）默认丢弃并记录告警，绝不执行。
3. 工具声明在导入时收敛到平台允许的工具白名单；白名单之外的工具声明直接拒绝导入项（fail-closed）。
4. 同 `identifier` 重复导入走「新建版本」而非覆盖；语义化版本相同则拒绝并提示。
5. 内置技能库目录按中间件域组织，保持现有 `builtin.Definition` 安装语义不变（平台级、已发布、不可直接修改）。
6. 导入操作要求 `skill:manage` 权限，写入审计日志；来源、原始摘要（SHA-256）和转换告警随版本保留。

## 3. 本迭代交付边界

计划交付：

- Skill 导入 API（`POST /api/v1/skills/import`）与前端导入入口（上传 zip / 粘贴文档）；
- `opsseek.skill/v1` 与 Agent Skills（SKILL.md）两种来源的解析、转换与校验；
- 工具白名单补齐（当前 `allowedToolName` 缺失 Oracle/Elasticsearch/RabbitMQ/MinIO 工具，详见 R001 §2.3）；
- 内置技能库按域重组与首批新技能（MongoDB 等）；
- 导入审计与默认禁用策略。

非目标（沿用 I005 §5）：

- 不允许导入内容改变工具白名单、目标资源、凭据或审批边界；
- 不执行导入包中的任何脚本/可执行文件；
- 不引入外部 Skill 市场或联网拉取；社区技能以人工整理进入 `builtin/`。

## 4. 需求清单

| 需求 | 名称 | 任务 | 状态 | 需求文档 | 验收报告 |
|---|---|---|---|---|---|
| R001 | Skills 导入与技能库整理 | T01-T06 | 设计评审中 | [R001-requirement.md](R001-requirement.md) | 待补充 |

## 5. 任务和依赖

| 任务 | 名称 | 依赖 | 主要产出 | 状态 |
|---|---|---|---|---|
| T01 | OpsKeeper 历史能力差异补齐 | 无 | 工具白名单补齐（Oracle/ES/RabbitMQ/MinIO）、差异清单确认 | 设计中 |
| T02 | Skill 导入核心服务 | T01 | 导入解析、格式转换（SKILL.md → opsseek.skill/v1）、校验、去重/版本策略 | 设计中 |
| T03 | 导入 API 与审计 | T02 | `POST /api/v1/skills/import`、`skill:manage` 守卫、审计事件 | 设计中 |
| T04 | 内置技能库按域重组 | T01 | `builtin/` 目录按中间件域分类、Definition 加载机制 | 设计中 |
| T05 | 首批新技能迁移 | T04 | MongoDB 健康诊断等首批技能（清单见 R001 §6） | 设计中 |
| T06 | 前端导入与管理入口 | T03 | 技能页导入向导、转换告警展示、默认禁用提示 | 设计中 |

## 6. 进入条件

- I005 交付的 Skill 独立领域模型、版本发布/禁用流程和权限守卫可用；
- `backend/skill/document.go` 的三格式解析与规范化（`ParseDocument`）可复用为导入解析基础；
- Engine 工具执行边界（`ResolvePlan` → `AllowedTools`）保持不变。

## 7. 退出条件

- 两种来源（OpsSeek 导出、Agent Skills 包）均可导入并生成 `draft` 版本；
- 导入的工具声明全部收敛于平台白名单，越界工具被拒绝且给出明确错误；
- 同 identifier 重复导入生成新版本，不可变版本历史完整可审计；
- 内置技能库按域组织，首批技能清单全部落地或明确转入 backlog；
- 导入技能默认不自动发布，启用需 `skill:manage` 人工操作并有审计记录。

## 8. 封板记录

待补充。
