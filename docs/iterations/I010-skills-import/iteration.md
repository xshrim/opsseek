# I010 Skills 迁移与导入

**迭代编号：** I010  
**目录标识：** I010-skills-import  
**状态：** 已批准（待实施）——设计评审（[R001-design-review.md](R001-design-review.md)，2026-11-18）阻塞项 B1-B3 已返修合入，裁定记录见 §9  
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
5. 内置技能库目录按中间件域组织，平台级、已发布、不可直接修改的语义不变；builtin 代码定义与 SQL 种子的权威来源关系按 M2 裁定在 A1/T04 中澄清（见 R001 §5）。
6. 导入操作要求 `skill:manage` 权限，写入审计日志；来源、原始摘要（SHA-256）和转换告警随版本保留。

## 3. 本迭代交付边界

计划交付：

- Skill 导入 API（`POST /api/v1/skills/import`）、导出 API（`GET .../export`）与前端导入入口（上传 zip / 粘贴文档）；
- `opsseek.skill/v1` 与 Agent Skills（SKILL.md）两种来源的解析、转换与校验，含 zip 解包安全（zip-slip/symlink/嵌套归档）与 YAML/正文炸弹防护；
- 前置修复（A1，先行独立合入）：工具白名单补齐（Oracle/Elasticsearch/RabbitMQ/MinIO/**OceanBase**）、builtin `Definition` 增加输出契约字段并补 OceanBase 定义、白名单与工具注册表一致性测试，详见 R001 §1.3；
- 内置技能库按域重组与首批新技能（清单见 R001 §6，MongoDB/MySQL 复制类转 backlog 待工具补齐）；
- 导入审计与默认禁用策略。

非目标（沿用 I005 §5）：

- 不允许导入内容改变工具白名单、目标资源、凭据或审批边界；
- 不执行导入包中的任何脚本/可执行文件；
- 不引入外部 Skill 市场或联网拉取；社区技能以人工整理进入 `builtin/`。

## 4. 需求清单

| 需求 | 名称 | 任务 | 状态 | 需求文档 | 验收报告 |
|---|---|---|---|---|---|
| R001 | Skills 导入与技能库整理 | A1-1~A1-3、A2-1、B2-1~B2-5 | 已批准（待实施） | [R001-requirement.md](R001-requirement.md) | 待补充 |

## 5. 任务和依赖

返修后任务重组为两条主线（原 T01-T06 映射见 §8 任务拆解）：

| 主线 | 名称 | 依赖 | 主要产出 | 状态 |
|---|---|---|---|---|
| A1 | 前置修复（先行独立合入） | 无 | 白名单补齐（含 OceanBase）、builtin Definition 输出契约字段、一致性测试 | 已批准（待实施） |
| A2 | MongoDB 诊断工具前置 | 无（与 A1 并行） | backend/tool 或诊断目标侧 MongoDB 只读诊断工具集 | 已批准（待实施，mongodb-health 技能的前置） |
| B2 | Skill 导入功能实施 | A1 | 迁移 0006、导入/导出 API、校验与防护、幂等与 draft 流程、前端 | 已批准（待实施） |

## 6. 进入条件

- I005 交付的 Skill 独立领域模型、版本发布/禁用流程和权限守卫可用；
- `backend/skill/document.go` 的三格式解析与规范化（`ParseDocument`）可复用为导入解析基础；
- Engine 工具执行边界（`ResolvePlan` → `AllowedTools`）保持不变。

## 7. 退出条件

- 两种来源（OpsSeek 导出、Agent Skills 包）均可导入并生成 `draft` 版本；OpsSeek 导出文档 round-trip 幂等；
- 导入的工具声明全部收敛于平台白名单，越界工具被拒绝且给出明确错误；
- zip-slip / symlink / 嵌套归档 / YAML 炸弹攻击包被拒，验收见 R001 §7；
- 同 identifier 重复导入生成新版本，不可变版本历史完整可审计；
- 内置技能库按域组织，首批技能清单（R001 §6，9 项）全部落地或明确转入 backlog；
- 导入技能默认不自动发布，启用需 `skill:manage` 人工操作并有审计记录。

## 8. 任务拆解

### A1 前置修复（先行独立合入，无依赖）

| 子任务 | 范围 | 依赖 | 验收标准 | 预估迁移 |
|---|---|---|---|---|
| A1-1 工具白名单补齐与单一来源 | `service.go` `allowedToolName` 补齐 oracle_/elasticsearch_/rabbitmq_/minio_/oceanbase_ 全量工具；白名单改由 `backend/tool/*` 注册表生成或加 CI 比对一致性测试（M1） | 无 | 五域技能版本可声明各自工具并通过 `CreateVersion`；一致性测试覆盖白名单 vs 注册表全量工具名 | 无 |
| A1-2 builtin Definition 输出契约字段 | `Definition` 结构增加 OutputSchema 字段；补 OceanBase builtin 定义（对齐 DB 种子 10 域）；新域技能种子补 0013 诊断输出契约（M2/N4/B1） | 无 | `Definition` 可携带输出契约；OceanBase 代码定义与种子一致；新域各版本 output_schema 非空且符合 0013 契约 | 无（沿用既有种子路径则无迁移；若需修种子则并入 0006） |
| A1-3 文档同步 | 更新 `docs/guides/integrations.md:29` 为 10 个域（含 OceanBase，N5） | A1-1/2 | 文档与实际一致 | 无 |

### A2 MongoDB 诊断工具前置（与 A1 并行，可延后）

| 子任务 | 范围 | 依赖 | 验收标准 | 预估迁移 |
|---|---|---|---|---|
| A2-1 MongoDB 只读诊断工具集 | 在 backend/tool 或诊断目标侧按既有域模式实现 MongoDB 只读诊断工具（复用 edbeb96 的资源 catalog），并纳入工具注册表与白名单 | A1-1（注册表/白名单机制） | 工具可被 Engine `ResolvePlan` 调用并收敛入 `AllowedTools`；mongodb-health 技能解除 backlog（技能本身在 B2-4 落地） | 视实现定（工具注册通常无迁移） |

### B2 导入功能实施（依赖 A1）

| 子任务 | 范围 | 依赖 | 验收标准 | 预估迁移 |
|---|---|---|---|---|
| B2-1 数据模型与导出 API | 迁移 **0006**（`skill_versions` 加 `source_type`/`source_digest`，部分唯一索引见 R001 §3.2.3）；`GET /api/v1/skills/{id}/versions/{vid}/export` 序列化 Document | A1 | 迁移可回滚（只加可空列）；导出文档可被导入 round-trip，SHA-256 一致 | **0006**（唯一迁移，已预分配） |
| B2-2 导入解析与安全防护 | zip 解包（zip-slip/symlink/嵌套归档/条目数 ≤200/解压比率 ≤100）、YAML 受限解析（SKILL.md ≤256KB、frontmatter ≤64KB、深度 ≤100）、SKILL.md → opsseek.skill/v1 转换与告警 | B2-1 | R001 §3.2.1/3.2.2/§7 攻击包用例全部拒绝且不落盘越界 | 无 |
| B2-3 导入 API、幂等与 draft 流程 | `POST /api/v1/skills/import`（`skill:manage` 守卫、逐项结果、审计事件 `skill.import`）；同 identifier 建新版本、字符串精确匹配拒重、SHA-256 幂等（DB 约束） | B2-2 | R001 §3.3/3.4 全部验收点通过；并发双导入幂等 | 无（并入 0006） |
| B2-4 内置技能库按域重组与首批技能 | `builtin/` 按域分文件（R001 §5）、community/ 规则与来源快照（Q5）；首批 9 项技能清单落地（R001 §6），mongodb-health 仅当 A2-1 完成时纳入 | A1 | 目录结构与 R001 §5 一致；首批技能安装为平台级已发布对象；A2 未完成则 mongodb-health 显式留 backlog | 无 |
| B2-5 前端导入与管理入口 | 技能页导入向导（上传/粘贴、逐项结果）、转换告警展示、发布前正文全文预览确认页（工具清单 + risk_level + 注入扫描告警）、import-report（限 `skill:manage`） | B2-3 | R001 §3.2/M4 验收：发布必经预览页；告警与审计可见 | 无 |

## 9. 裁定记录

**日期：2026-11-18。** 设计评审（[R001-design-review.md](R001-design-review.md)，结论「有条件通过」）与联合评审（[I008-I010-joint-review.md](../I008-I010-joint-review.md)）意见已合入，用户已批准推荐裁定：

- **B1**：白名单与 builtin 均补入 OceanBase 域（A1-1/A1-2）；9 vs 10 不一致以代码定义补齐对齐种子。
- **B2**：zip 解包与 YAML/正文炸弹防护写入 R001 §3.2.1/3.2.2 为明确 FR 与验收标准（B2-2）。
- **B3**：mongodb-health 移出首批清单，改为「先补 MongoDB 诊断工具再上技能」的独立前置子任务（A2）；首批清单重排为 9 项；mysql-replication-health 同转 backlog（M6）。
- **声称收窄（N1）**：「0013 契约未随迁」修正为「旧域部分版本保留、全部新域版本与 builtin 代码定义缺失」，前置修复范围=Definition 增输出契约字段+新域补齐（A1-2）。
- **M1-M6、N1-N7** 逐条采纳情况见 R001 §8.1（全部采纳；N7 联网复核 owner=用户）。
- **开放问题 1-6** 全部按评审建议裁定，见 R001 §8。
- **跨迭代**：A1 先行独立合入（与 I008 无重叠）；本迭代迁移编号预分配 0006。
- 原 T01→A1，T02/T03/T06→B2-2/B2-3/B2-5，T04→A1-2+B2-4，T05→B2-4；新增 A2。

## 10. 封板记录

待补充。
