# R001 Skills 导入与技能库整理 — 需求分析

**所属迭代：** [I010 Skills 迁移与导入](iteration.md)  
**状态：** 设计评审中  
**编写日期：** 2026-11-18

## 1. 现状盘点

### 1.1 当前 Skill 模型

- **领域对象**：`Skill`（id、scope_id、name、identifier、category、tags、maintainer、status）见 `backend/skill/types.go:10-21`；四类 category：diagnosis/monitoring/optimization/maintenance（`types.go:23-27`）。
- **可迁移文档契约**：`Document{api_version: opsseek.skill/v1, kind: Skill, metadata{name,identifier,version,category,tags,maintainer}, spec{description,input_schema,output_schema,body,tools,dependencies}}`（`types.go:39-53`，spec 字段白名单见 `document.go:202`）。`ParseDocument` 统一解析 JSON/YAML/Markdown（frontmatter + 正文进 `spec.body`）并做规范化 + SHA-256 摘要（`document.go:22-57`）。**该契约已是良好的导入载体，导入功能可直接复用。**
- **版本**：`Version` 含 Manifest（name/description/instruction）、input/output schema、Tools 白名单（≤20，`service.go:54`）、risk_level（read_only/controlled/high，`service.go:77-79`）、status（draft→published）。
- **权限与 API**：`skill:read/manage/execute`（`backend/authorization/types.go:23-25`）；现有 API 仅有 `GET /skills`、`POST /skills/{id}/versions`、publish/disable（`backend/httpapi/skill.go:194-223`）。**没有导入和导出 API。**
- **内置技能**：`backend/skill/builtin/definitions.go:25-33` 共 9 个健康诊断 Skill（Kubernetes、PostgreSQL、MySQL、Oracle、Redis、Kafka、Elasticsearch、RabbitMQ、MinIO），每个绑定 Connector Capability 与只读工具集，作为不可变声明安装为平台级已发布 Skill。

### 1.2 OpsKeeper 时期（git 历史）能力对比

| 能力 | OpsKeeper 时期 | 现状 | 差异结论 |
|---|---|---|---|
| 内置技能数 | 4 个（K8s/PG/Redis/Kafka），commit `c7ddc64`（`git show c7ddc64:backend/skill/builtin/definitions.go`） | 9 个（新增 MySQL/Oracle/ES/RabbitMQ/MinIO） | 扩充，无丢失 |
| 单域工具粒度 | 每域 1 个 inspect 快照工具（如 `connector_postgresql_inspect`） | 每域拆成 6-9 个细粒度只读工具 | 细化，无丢失 |
| Manifest.TargetKinds | 存在（`c7ddc64` definitions.go、0013 迁移） | 已移除 | I005 决策 6 明确弃用（目标上下文由资源/Tool Catalog/权限决定），非遗漏 |
| 结构化输出契约 | 0013 迁移内置固定输出 schema（facts/findings/evidence/hypotheses/confidence/recommendations），见 `git show c7ddc64:backend/migrations/sql/0013_builtin_skills.sql` | 输出契约移到 Version.output_schema，由创建者声明 | 迁移到通用机制，但内置技能的**标准诊断输出契约未随迁到当前 builtin 定义**——可作为导入/内置技能标准化的一部分补回（待验证当前 builtin 安装迁移是否携带 output contract） |
| LLM skill runner | 独立 `backend/skill/runner.go`（commit `4d8454f`，431 行） | 文件已不存在；由 AIEngine 统一执行（commit `c0029b0` refactor: unify execution through AIEngine） | 架构收敛，能力并入 engine，无丢失 |
| Skill 存储 | 作为 `resources` 表 kind='Skill'（0013 迁移） | 独立 `skills`/`skill_versions` 表 | 领域拆分（I005），无丢失 |

### 1.3 发现的现存缺口（导入功能的前置修复）

**工具白名单未同步扩充**：`backend/skill/service.go:255-262` 的 `allowedToolName` 只包含 connector 通用工具和 PostgreSQL/MySQL/Redis/Kafka 细粒度工具，**缺少 Oracle（oracle_health 等 7 个）、Elasticsearch（7 个）、RabbitMQ（9 个）、MinIO（6 个）工具**——这些工具在 `builtin/definitions.go:28-33` 中被内置技能使用，但用户自建/导入的 Skill 版本无法声明它们。导入功能上线前必须补齐，否则约一半中间件域的技能无法导入。

### 1.4 文档漂移

`docs/guides/integrations.md:29` 仍写「内置 Skill 当前覆盖 Kubernetes、PostgreSQL、Redis 和 Kafka」，与实际 9 个不一致，随本迭代更新。

## 2. 开源调研

### 2.1 Agent Skills 开放标准（重点采纳对象）

来源：https://agentskills.io/specification.md 、https://www.anthropic.com/research/equipping-agents-for-the-real-world-with-agent-skills （2025-10 发布，2025-12 开放标准化）。

格式特征：

- **目录约定**：一个 Skill = 一个目录，必须含 `SKILL.md`；可选 `scripts/`（可执行代码）、`references/`（参考文档）、`assets/`（模板/静态资源）。
- **Frontmatter 字段**：必填 `name`（≤64 字符，小写字母数字连字符，须与目录名一致）、`description`（≤1024 字符）；可选 `license`、`compatibility`（≤500 字符）、`metadata`（string→string 映射，常放 author/version）、`allowed-tools`（空格分隔的预批准工具，实验性，如 `Bash(git:*) Read`）。
- **渐进披露**：元数据（~100 token）→ SKILL.md 正文（建议 <5000 token / 500 行）→ 按需加载资源文件。
- **校验**：官方 `skills-ref validate` 工具。
- **版本化**：标准本身不强制版本，社区惯例放 `metadata.version`。

### 2.2 可收集的开源技能库

- `anthropics/skills`（https://github.com/anthropics/skills）：官方技能仓库，以文档处理/工件类为主，运维类较少。
- `ComposioHQ/awesome-claude-skills`（https://github.com/ComposioHQ/awesome-claude-skills）、`VoltAgent/awesome-agent-skills`（https://github.com/VoltAgent/awesome-agent-skills，1000+ 技能）：社区精选清单，含数据库类技能（如 PostgreSQL 只读查询技能）。
- `awesome-skills.com`、`bestclaudecodeskills.com/categories/databases`（34 个数据库类技能）：目录站，可作为选材来源。

注意：社区 Claude Code 技能大量依赖 `Bash`/脚本执行，与 OpsSeek「不允许正文或导入包执行任意命令」的边界（I005 非目标）冲突，**只能借鉴其知识正文（runbook、诊断步骤），不能直接执行其 scripts/**。具体技能内容的可用性：待验证。

### 2.3 格式映射：Agent Skills → opsseek.skill/v1

| SKILL.md | opsseek.skill/v1 | 处置 |
|---|---|---|
| `name`（≤64） | `metadata.identifier`（`^[a-z0-9][a-z0-9.-]{2,119}$`，`document.go:15`） | 直接映射（连字符合法）；若与目录名不一致以文件为准并告警 |
| `description`（≤1024） | `metadata.name` 与 `spec.description` | `metadata.name` 要求无空白单词（`document.go:238-240`）且 ≤200 字符，取 name；超长 description 截断到 4000 上限内可容纳，直接映射到 `spec.description` |
| `metadata.version` | `metadata.version` | 缺失时默认 `0.1.0`（草稿语义） |
| `metadata.author` | `metadata.maintainer` | 缺失时填 `imported:<actorID>`，发布前人工补全 |
| `metadata` 其余键 | `metadata.tags`（≤50 项，`document.go:255`） | 白名单键（如 domain）映射，其余丢弃并告警 |
| `compatibility` | 不映射 | 丢弃并告警 |
| `license` | 暂无字段 | 记录到导入审计；是否增加 metadata 字段见 §8 开放问题 |
| `allowed-tools` | `spec.tools` | 逐个匹配平台工具白名单：匹配成功→映射为 ToolSpec；匹配失败→**拒绝该项**（fail-closed，不允许「留空待运行时解释」）。Agent Skills 工具名（Bash/Read 等）与 OpsSeek 工具名不同，提供映射表（如 `Bash(psql:*)`→postgresql_* 不自动映射，需人工选择，见 §8） |
| Markdown 正文 | `spec.body` | 直接映射；剥离 HTML/script 标签等不安全内容（正文只作为模型指令，不执行） |
| `input_schema`/`output_schema` | Agent Skills 无此概念 | 导入后默认 `{"type":"object"}`，需人工补全后才建议发布 |
| `scripts/`、`assets/` | 无对应 | **不导入、不存储、不执行**，记录告警；`references/*.md` 可拼接进 body 或作为 body 附加段落（≤500 行建议） |
| category | 无对应 | 必填项，导入向导中人工选择（默认 diagnosis） |

## 3. 导入功能需求

### 3.1 导入来源与格式

1. **OpsSeek 导出 JSON/YAML/Markdown**：即 `opsseek.skill/v1` 文档，直接走 `ParseDocument` 校验后落库（一对一保真，工具白名单仍需校验）。
2. **Agent Skills 目录 / zip 包**：zip 解包后按 §2.3 转换。包内可含多个技能目录，逐个处理并返回逐项结果（部分成功允许）。
3. （后续可选）粘贴单份 SKILL.md 文本——与 zip 共用转换逻辑。

### 3.2 校验规则

- **权限**：导入者须对目标 scope 拥有 `skill:manage`；目标 scope 默认为导入者当前管理 scope。
- **工具收敛**：转换后 `spec.tools`/`version.Tools` 每个名字必须命中 `allowedToolName`（补齐后）；任何未命中即该项导入失败并返回具体工具名。
- **正文安全**：Markdown body 过滤 `<script>`、`javascript:` 链接等注入向量；长度限制沿用现有校验；引用的相对路径文件不内联执行。
- **资源限制**：zip ≤10MB、解包 ≤50MB、技能目录数 ≤20/包、body ≤100KB（防止资源耗尽，数值待评审）。
- **文档校验**：复用 `ValidateDocument` 全量规则（identifier/version/category/tags 等）。

### 3.3 版本化与去重策略

- identifier 在目标 scope 内已存在 → 在既有 Skill 下创建**新版本**（版本号取导入 metadata.version；与现有版本语义化版本相同则拒绝）。
- identifier 不存在 → 创建 Skill（status=draft）+ 版本 v1（**draft，不自动发布**，与内置技能的「安装即发布」不同——导入内容不可信，须人工审查后 publish）。
- 保留 `ContentSHA256`（`document.go:49-55` 已有摘要机制）作为同内容幂等键：同 identifier + 同 SHA-256 重复导入直接返回已有版本（幂等）。

### 3.4 API 变更

- `POST /api/v1/skills/import`（multipart：zip/文档文件 + 目标 scope_id + category 覆盖项；`skill:manage` 守卫）→ 返回逐项 `{status: imported/skipped/failed, skill_id, version_id, warnings[]}`。
- `GET /api/v1/skills/{id}/versions/{vid}/import-report`（可选）→ 查看转换告警与原始摘要。
- 审计事件：`skill.import`（记录来源类型、包摘要、逐项结果、actor）。

### 3.5 数据模型变更

- `skill_versions` 增加 `source_type`（builtin/imported/manual，默认 manual）与 `source_digest`（原始包 SHA-256），nullable，向后兼容；（若评审采纳 license 记录）`manifest` JSON 中非结构化保留即可，不加列。
- 无需新表。

## 4. 治理与安全默认

1. **默认禁用**：导入版本一律 draft；发布动作沿用现有 `skill:manage` + publish 端点，天然满足「人工启用」。
2. **scope 归属**：导入只能落在导入者可管理的 scope；平台级导入仅平台管理员；跨 scope 复制沿用既有复制机制。
3. **审计**：导入、发布均落审计日志（现有 auditor 注入 `httpapi/skill.go:137`）。
4. **不执行原则**：导入包中的可执行内容（scripts/、allowed-tools 之外的声明）一律丢弃/拒绝并告警，与 I005 非目标一致。

## 5. 内置/社区技能库组织

`backend/skill/builtin/` 扩展为按域分文件（保持 Go 包不变）：

```text
backend/skill/builtin/
├── definitions.go        # 聚合入口 Definitions() []Definition
├── kubernetes.go         # K8s 域技能
├── rdbms.go              # PostgreSQL / MySQL / Oracle
├── cache.go              # Redis
├── messaging.go          # Kafka / RabbitMQ
├── search_storage.go     # Elasticsearch / MinIO / MongoDB
└── community/            # 从开源收集整理的技能（人工评审后进入，仅知识正文+平台工具）
```

规则：`community/` 中的技能同样只能声明 `allowedToolName` 白名单内工具，安装后仍为「复制后可改」的平台级对象；开源内容的许可证在文件头注明。

## 6. 首批迁移/收集技能清单

以现有诊断能力为基础扩展，避免与 9 个既有 health 技能重复：

| # | 技能 | 域 | category | 主要工具（已有） | 来源 |
|---|---|---|---|---|---|
| 1 | mongodb-health | MongoDB | diagnosis | 依赖 MongoDB catalog 工具（commit edbeb96 新增，工具名待验证） | 自研 |
| 2 | kubernetes-events-troubleshooting | K8s | diagnosis | connector_kubernetes_read（聚焦事件/探针失败归因） | 自研 |
| 3 | postgresql-slow-query-analysis | PostgreSQL | diagnosis | postgresql_sessions/long_running_queries/locks/performance | 自研 |
| 4 | postgresql-vacuum-advisor | PostgreSQL | optimization | postgresql_vacuum/tables/performance | 自研 |
| 5 | mysql-replication-health | MySQL | diagnosis | mysql_status/performance（复制项待验证） | 自研 |
| 6 | redis-memory-analysis | Redis | diagnosis | redis_memory/slowlog/clients | 自研 |
| 7 | kafka-consumer-lag-triage | Kafka | diagnosis | kafka_consumer_lag/consumer_groups/topics | 自研 |
| 8 | elasticsearch-index-sizing | ES | optimization | elasticsearch_indices/shards/settings | 自研 |
| 9 | minio-bucket-hygiene | MinIO | maintenance | minio_buckets/versioning/lifecycle | 自研 |
| 10 | k8s-capacity-review | K8s | monitoring | connector_kubernetes_read + connector_metrics_query | 自研 |
| 11 | db-backup-readiness-check | 通用数据库 | maintenance | 各域只读工具 | 参考 awesome-claude-skills 数据库类 runbook 改写（待验证可用内容） |
| 12 | 通用诊断输出契约模板 | 通用 | — | — | 恢复 OpsKeeper 0013 迁移的 facts/findings/evidence/hypotheses/confidence/recommendations 契约，作为所有诊断类技能的默认 output_schema（`git show c7ddc64:backend/migrations/sql/0013_builtin_skills.sql`） |

注：3/4/6/7 所需的 postgresql_sessions、postgresql_long_running_queries、postgresql_locks、postgresql_replication、postgresql_capacity 已在白名单（`service.go:257`）。

## 7. 验收要点（评审时细化为任务验收）

- 导入 OpsSeek 导出文档 round-trip：SHA-256 一致、版本幂等。
- 导入含越界工具的 SKILL.md：该技能项失败且错误包含工具名；同包其他技能成功。
- 导入含 scripts/ 的包：不落库脚本、不执行、告警可见。
- 导入版本均为 draft；发布后可被 Engine `ResolvePlan` 执行且 `AllowedTools` 收敛正确。
- 白名单补齐后，Oracle/ES/RabbitMQ/MinIO 域技能可正常创建/导入。
- 所有导入操作在审计日志可查。

## 8. 开放问题

1. `allowed-tools`（Agent Skills）与 OpsSeek 工具名语义差异大：是否提供人工映射向导（导入时为每个外部工具选择平台工具）而非仅 fail-closed？待评审。
2. license/来源字段是否升级为 `Metadata` 正式字段而非依赖审计记录？待评审。
3. `references/` 内容是否内联进 body（撑大上下文）还是作为 body 附件分页存（需要 document 格式扩展 `opsseek.skill/v2`）？建议 v1 先内联并限制行数。
4. MongoDB 工具命名与 Capability 是否沿用 edbeb96 的 catalog 模式、以及对应只读工具集名称——待验证。
5. 社区技能是否需要「来源仓库与 commit 快照」记录以便追溯更新？建议 T04 时定。
6. 是否提供 Skill 导出 API（作为迁移闭环）？当前仅有导入设计，导出用文档契约直接序列化即可，工作量小，倾向纳入 T02。

## 参考来源

- Agent Skills 规范：https://agentskills.io/specification.md
- Anthropic Agent Skills 发布：https://www.anthropic.com/research/equipping-agents-for-the-real-world-with-agent-skills
- 官方技能仓库：https://github.com/anthropics/skills
- 社区集合：https://github.com/ComposioHQ/awesome-claude-skills 、https://github.com/VoltAgent/awesome-agent-skills 、https://awesome-skills.com 、https://bestclaudecodeskills.com/categories/databases
