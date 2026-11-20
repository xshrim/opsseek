# R001 Skills 导入与技能库整理 — 需求分析

**所属迭代：** [I010 Skills 迁移与导入](iteration.md)  
**状态：** 已批准（待实施）——评审意见已合入，见 iteration.md §9 裁定记录  
**编写日期：** 2026-11-18（返修 2026-11-18，合入 [R001-design-review.md](R001-design-review.md) 阻塞项 B1-B3、主要 M1-M6、次要 N1-N7 裁定）

## 1. 现状盘点

### 1.1 当前 Skill 模型

- **领域对象**：`Skill`（id、scope_id、name、identifier、category、tags、maintainer、status）见 `backend/skill/types.go:10-21`；四类 category：diagnosis/monitoring/optimization/maintenance（`types.go:23-27`）。
- **可迁移文档契约**：`Document{api_version: opsseek.skill/v1, kind: Skill, metadata{name,identifier,version,category,tags,maintainer}, spec{description,input_schema,output_schema,body,tools,dependencies}}`（`types.go:39-53`，spec 字段白名单见 `document.go:202`）。`ParseDocument` 统一解析 JSON/YAML/Markdown（frontmatter + 正文进 `spec.body`）并做规范化 + SHA-256 摘要（`document.go:22-57`）。**该契约已是良好的导入载体，导入功能可直接复用。**
- **版本**：`Version` 含 Manifest（name/description/instruction）、input/output schema、Tools 白名单（≤20，`service.go:54`）、risk_level（read_only/controlled/high，`service.go:77-79`）、status（draft→published）。
- **权限与 API**：`skill:read/manage/execute`（`backend/authorization/types.go:23-25`）；现有 API 仅有 `GET /skills`、`POST /skills/{id}/versions`、publish/disable（`backend/httpapi/skill.go:194-223`）。**没有导入和导出 API。**
- **内置技能**：`backend/skill/builtin/definitions.go:25-33` 共 9 个健康诊断 Skill（Kubernetes、PostgreSQL、MySQL、Oracle、Redis、Kafka、Elasticsearch、RabbitMQ、MinIO），每个绑定 Connector Capability 与只读工具集。**注意：SQL 种子（`0001_initial.sql:2073-2100`）实际含 10 个技能（多一个 OceanBase），代码定义与 DB 种子不一致（9 vs 10），本迭代 A1 前置修复对齐（见 §1.3）。**

### 1.2 OpsKeeper 时期（git 历史）能力对比

| 能力 | OpsKeeper 时期 | 现状 | 差异结论 |
|---|---|---|---|
| 内置技能数 | 4 个（K8s/PG/Redis/Kafka），commit `c7ddc64`（`git show c7ddc64:backend/skill/builtin/definitions.go`） | 9 个（新增 MySQL/Oracle/ES/RabbitMQ/MinIO） | 扩充，无丢失 |
| 单域工具粒度 | 每域 1 个 inspect 快照工具（如 `connector_postgresql_inspect`） | 每域拆成 6-9 个细粒度只读工具 | 细化，无丢失 |
| Manifest.TargetKinds | 存在（`c7ddc64` definitions.go、0013 迁移） | 已移除 | I005 决策 6 明确弃用（目标上下文由资源/Tool Catalog/权限决定），非遗漏 |
| 结构化输出契约 | 0013 迁移内置固定输出 schema（facts/findings/evidence/hypotheses/confidence/recommendations），见 `git show c7ddc64:backend/migrations/sql/0013_builtin_skills.sql` | 输出契约移到 Version.output_schema，由创建者声明 | 评审核验结论（收窄）：**部分旧域版本保留、全部新域版本及 builtin 代码定义缺失**——当前合并基线 `0001_initial.sql:2073-2081` 中 K8s/PG/Redis/Kafka 各版本保留了完整契约；丢失契约的是 MySQL/ES/Oracle/RabbitMQ/MinIO/OceanBase 各 v1 及 Kafka v3（`output_schema='{}'`），且 `builtin.Definition` 结构体无 OutputSchema 字段（类型层面无法携带契约）、`Definitions()` 无生产调用方（内置技能实际由 SQL 种子安装）。前置修复范围相应调整为「builtin Definition 增加输出契约字段 + 新域种子补齐契约」 |
| LLM skill runner | 独立 `backend/skill/runner.go`（commit `4d8454f`，431 行） | 文件已不存在；由 AIEngine 统一执行（commit `c0029b0` refactor: unify execution through AIEngine） | 架构收敛，能力并入 engine，无丢失 |
| Skill 存储 | 作为 `resources` 表 kind='Skill'（0013 迁移） | 独立 `skills`/`skill_versions` 表 | 领域拆分（I005），无丢失 |

### 1.3 发现的现存缺口（导入功能的前置修复，A1 先行独立合入）

**工具白名单未同步扩充（B1，含 OceanBase 遗漏）**：`backend/skill/service.go:255-262` 的 `allowedToolName` 只包含 connector 通用工具和 PostgreSQL/MySQL/Redis/Kafka 细粒度工具，**缺少 Oracle（7 个）、Elasticsearch（7 个）、RabbitMQ（9 个）、MinIO（6 个）、OceanBase（7 个）工具**——这些工具在 `builtin/definitions.go:28-33`（OceanBase 除外）中被内置技能使用，且 DB 种子（`0001_initial.sql:2095`）已有 OceanBase 已发布技能。限定条件（评审核验）：`CreateVersion` 是唯一走 `allowedToolName` 的路径，内置技能经 SQL 种子直接落库不经该校验，因此缺口影响面是「用户自建/导入版本」。此外 builtin 代码定义缺 OceanBase 一域（9 vs 10 不一致），A1 需一并补齐并澄清代码定义与 SQL 种子的权威来源关系（M2，目标态见 §5）。

**白名单与工具注册表双源漂移（M1，采纳为需求）**：白名单为手工 switch，与 `backend/tool/*` 工具注册表双源维护，本次五个域的缺失正是漂移证据。A1 中增加一致性保障：白名单改由工具注册表生成，或在 CI 中比对两源并加一致性测试，收敛为单一来源。

**内置技能诊断输出契约缺失（见 §1.2 收窄结论）**：A1 中为 `builtin.Definition` 增加 OutputSchema 字段，并为全部缺失契约的新域版本补齐 0013 契约。

**跨迭代协调**：A1 前置修复（白名单补齐 + builtin 输出契约字段）按联合评审裁定（[I008-I010-joint-review.md](../I008-I010-joint-review.md) §6 事项 4）**先行独立合入 main**，与 I008 工具网关改造无代码重叠（I008 改调用路径，本修复改 Skill 域校验函数）。本迭代主功能迁移编号预分配 **0006**（当前 `backend/migrations/sql/` 非归档迁移最大 0003；0004 归 I008、0005 归 I009；`archive/` 不动）。

### 1.4 文档漂移

`docs/guides/integrations.md:29` 仍写「内置 Skill 当前覆盖 Kubernetes、PostgreSQL、Redis 和 Kafka」，与实际不一致；更新时按 DB 种子实情写 **10 个域（含 OceanBase）**（N5 采纳）。

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
| `metadata` 其余键 | `metadata.tags`（≤50 项，`document.go:255`） | 白名单键（如 domain）映射；**嵌套/非 string 值显式拒绝该项并告警**（N6 采纳），其余丢弃并告警 |
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
- **工具收敛**：转换后 `spec.tools`/`version.Tools` 每个名字必须**逐字精确匹配** `allowedToolName`（无通配、无前缀、无大小写折叠，M4/S4 采纳）；Agent Skills 的 `Bash(psql:*)` 形式必须解析出完整 token 后整体判失败，不允许截断到 `Bash` 再模糊匹配。任何未命中即该项导入失败并返回具体工具名。
- **正文安全**：Markdown body 过滤 `<script>`、`javascript:` 链接等注入向量；长度限制沿用现有校验；引用的相对路径文件不内联执行。发布前审查界面固化「正文全文 diff/预览」为必经环节，并对已知注入模式（ignore previous、system prompt、凭据读取类关键词）做静态扫描告警（进 import-report，非拦截）；发布确认页展示工具清单与 risk_level（M4/S3 采纳）。
- **资源限制**：zip ≤10MB、解包 ≤50MB、技能目录数 ≤20/包、body ≤100KB、单文件条目数 ≤200/包。
- **文档校验**：复用 `ValidateDocument` 全量规则（identifier/version/category/tags 等）。

#### 3.2.1 zip 解包安全（B2/S1 采纳，FR）

解压实现必须满足以下防护，全部为验收硬标准：

1. **zip-slip 路径穿越防护**：解压每个条目前，将条目名清洗后与解包目标根做路径归一化校验（`filepath.Clean` + `filepath.Rel` / 前缀检查）；**拒绝绝对路径条目、拒绝含 `..` 的条目**，命中即整个包导入失败。
2. **符号链接拒绝**：检测 zip 条目 mode 中的 symlink 位，**拒绝 symlink 条目**（含硬链接）。
3. **不递归解包嵌套归档**：zip 内再出现 zip/tar/gz 等归档文件不解包，作为普通文件按大小限额处理；`.zip` 条目直接丢弃并告警。
4. **解包目标隔离**：解压到一次性临时目录（`os.MkdirTemp`），处理完毕即删除；不允许写到仓库或数据目录。
5. **条目数上限**：每包 ≤200 个文件条目（含目录条目），超限拒绝整包。

#### 3.2.2 YAML/正文炸弹防护（M3/S2 采纳，FR）

1. **解析前大小限制**：单个 SKILL.md（含 frontmatter）≤256KB，frontmatter 段单独 ≤64KB；超限拒绝该项。
2. **受限解析器**：导入路径的 YAML 解析使用 `yaml.Decoder` 流式解析并限制别名/锚点使用（拒绝 alias 展开超过阈值、嵌套深度 ≤100），**不使用** `yaml.Unmarshal` 整包解析（现有 `document.go:69` 的 Unmarshal 仅用于 OpsSeek 导出文档且已有 100KB body 限额兜底，导入路径前置截断后再进入）。
3. **解压比率上限**：单条目解压后大小 / 压缩大小比率 >100 直接拒绝该条目（典型 zip 炸弹比率 1000x+）。
4. **正文长度**：body ≤100KB（沿用 §3.2），frontmatter 解析失败按普通 YAML 错误拒绝该项，不重试。

#### 3.2.3 幂等与并发（M5/S6 采纳）

同 SHA-256 幂等除应用层查重外，必须有 DB 层保障：`skill_versions(skill_id, source_digest)` 部分唯一索引（source_digest 非空时），或事务内 `SELECT ... FOR UPDATE`；并发双导入至多产生一个版本，另一方返回已有版本。

### 3.3 版本化与去重策略

- identifier 在目标 scope 内已存在 → 在既有 Skill 下创建**新版本**（版本号取导入 metadata.version；版本冲突判定为**字符串精确匹配**即拒绝，不做语义比较——`1.0.0-alpha` 与 `1.0.0` 视为不同版本，N2 采纳）。
- identifier 不存在 → 创建 Skill（status=draft）+ 版本 v1（**draft，不自动发布**，与内置技能的「安装即发布」不同——导入内容不可信，须人工审查后 publish）。
- 保留 `ContentSHA256`（`document.go:49-55` 已有摘要机制）作为同内容幂等键：同 identifier + 同 SHA-256 重复导入直接返回已有版本（幂等）。

### 3.4 API 变更

- `POST /api/v1/skills/import`（multipart：zip/文档文件 + 目标 scope_id + category 覆盖项；`skill:manage` 守卫）→ 返回逐项 `{status: imported/skipped/failed, skill_id, version_id, warnings[]}`。
- `GET /api/v1/skills/{id}/versions/{vid}/import-report`（可选）→ 查看转换告警与原始摘要；**原始告警与导入报告仅 `skill:manage` 可见**（N3 采纳），draft 正文可见性沿用现状（`skill:read` 可见，可接受）。
- `GET /api/v1/skills/{id}/versions/{vid}/export`→ 序列化 `opsseek.skill/v1` Document 返回（开放问题 6 已裁定采纳，纳入 T02，round-trip 验收依赖）。
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

**M2 目标态澄清（评审核验：`Definitions()` 无生产调用方，内置技能实际由 SQL 种子安装，原 iteration.md §2.5「保持安装语义不变」与现状不符）**：本迭代 T04 交付将 builtin 明确为**代码定义是权威来源、SQL 种子仅作初始安装**的方向——A1 为 `Definition` 增加 OutputSchema 与域来源注释字段并补 OceanBase 定义；T04 落地按域分文件重组，并使新域种子/安装路径与代码定义一致（是否切回代码安装由 A1 实施时按改动面最终确定，不影响契约字段先行）。**诊断输出契约模板**（原清单 #12，N4 采纳移出技能清单）作为 T04 的契约资产落地：诊断类 builtin 技能默认 output_schema 恢复 0013 契约（facts/findings/evidence/hypotheses/confidence/recommendations），并为 `Definition` 结构补 OutputSchema 字段承载。

## 6. 首批迁移/收集技能清单

以现有诊断能力为基础扩展，避免与既有 health 技能重复。**返修调整（B3/M6/N4）**：原 #1 mongodb-health 依赖不存在的 MongoDB 诊断工具（评审核验：`backend/tool`、`backend/connector`、`backend/engine` 均无 mongodb 工具，edbeb96 仅新增资源目录，迁移 0003）——裁定为**先在 backend/tool 或诊断目标侧补 MongoDB 诊断工具、再上技能**，作为独立前置子任务并显式标注依赖（见 iteration.md 任务拆解 A2），未补工具前该技能不进入本迭代交付；原 #5 mysql-replication-health 因 `mysql_replication` 工具不存在同样转 backlog（`mysql_status` 是否含复制指标待工具侧验证，M6 同 #1 一并裁定）；原 #12 契约模板移入 §5 T04 契约资产。调整后清单重新编号为 1-9：

| # | 技能 | 域 | category | 主要工具（已有） | 来源 |
|---|---|---|---|---|---|
| 1 | kubernetes-events-troubleshooting | K8s | diagnosis | connector_kubernetes_read（聚焦事件/探针失败归因） | 自研 |
| 2 | postgresql-slow-query-analysis | PostgreSQL | diagnosis | postgresql_sessions/long_running_queries/locks/performance | 自研 |
| 3 | postgresql-vacuum-advisor | PostgreSQL | optimization | postgresql_vacuum/tables/performance | 自研 |
| 4 | redis-memory-analysis | Redis | diagnosis | redis_memory/slowlog/clients | 自研 |
| 5 | kafka-consumer-lag-triage | Kafka | diagnosis | kafka_consumer_lag/consumer_groups/topics | 自研 |
| 6 | elasticsearch-index-sizing | ES | optimization | elasticsearch_indices/shards/settings（依赖 A1 白名单补齐） | 自研 |
| 7 | minio-bucket-hygiene | MinIO | maintenance | minio_buckets/versioning/lifecycle（依赖 A1 白名单补齐） | 自研 |
| 8 | k8s-capacity-review | K8s | monitoring | connector_kubernetes_read + connector_metrics_query | 自研 |
| 9 | db-backup-readiness-check | 通用数据库 | maintenance | 各域只读工具 | 参考 awesome-claude-skills 数据库类 runbook 改写（可用内容待验证） |

**backlog（不在本迭代交付）**：mongodb-health（待 MongoDB 诊断工具，A2 前置子任务）、mysql-replication-health（待 mysql_replication 工具或验证 mysql_status 复制指标）。

注：3/4/6/7 所需的 postgresql_sessions、postgresql_long_running_queries、postgresql_locks、postgresql_replication、postgresql_capacity 已在白名单（`service.go:257`）。

## 7. 验收要点（评审时细化为任务验收）

- 导入 OpsSeek 导出文档 round-trip：SHA-256 一致、版本幂等；并发双导入至多产生一个版本。
- 导入含越界工具的 SKILL.md：该技能项失败且错误包含工具名；同包其他技能成功。
- 导入含 scripts/ 的包：不落库脚本、不执行、告警可见。
- **zip-slip 攻击包**（含 `../` 或绝对路径条目）整包拒绝，无任何文件落到目标目录之外；symlink 条目拒绝；嵌套 zip 不解包。
- **YAML 炸弹包**（超大 frontmatter / alias 爆炸 / 高压缩比条目）在解析前被拒，不触发 OOM 或长时阻塞。
- 导入版本均为 draft；发布需经正文预览确认页（含工具清单与 risk_level）；发布后可被 Engine `ResolvePlan` 执行且 `AllowedTools` 收敛正确。
- 白名单补齐后，Oracle/ES/RabbitMQ/MinIO/OceanBase 域技能可正常创建/导入；白名单与工具注册表一致性测试通过。
- 所有导入操作在审计日志可查；import-report 仅 `skill:manage` 可见。

## 8. 开放问题（已裁定，2026-11-18，随评审意见合入）

1. **allowed-tools 人工映射向导**：已裁定——v1 不做向导，保持纯 fail-closed；向导列入 backlog。理由：向导会把「外部任意工具声明」变成「人工背书的白名单扩边」，削弱 fail-closed 语义；社区技能本就只能借鉴知识正文，工具集应人工重选（等价于手工重建）。
2. **license 升级为 Metadata 正式字段**：已裁定——v1 不加字段（metadata rejectUnknown 会拒绝，加字段=改契约），license 记入 import-report + 审计；与来源快照一并考虑进未来 v2。
3. **references/ 内联 or 附件**：已裁定——v1 内联并限行数（≤500 行/文件，总量并入 body 100KB 限额）；超限丢弃并告警；不做 `opsseek.skill/v2` 扩展。
4. **MongoDB 工具命名**：已裁定——工具不存在（评审已验证），mongodb-health 不进入本迭代交付；改为独立前置子任务「先在 backend/tool 或诊断目标侧补 MongoDB 诊断工具，再上技能」（iteration.md 任务拆解 A2），未补工具前保持 backlog。
5. **社区技能来源仓库 + commit 快照**：已裁定——采纳，B2-4 落地为 Definition 增加来源注释或旁注文件。
6. **Skill 导出 API**：已裁定——采纳，纳入 B2-1（`GET /api/v1/skills/{id}/versions/{vid}/export`）。

## 8.1 评审问题采纳记录

| 问题 | 采纳情况 | 落点 |
|---|---|---|
| B1 OceanBase 遗漏 / 9 vs 10 不一致 | 采纳 | §1.1、§1.3、A1 子任务 |
| B2 zip 解包安全 | 采纳 | §3.2.1（FR + 验收） |
| B3 MongoDB 技能依赖不存在工具 | 采纳（推荐方案：先补工具再上技能，独立前置子任务） | §6、§8 问题 4、A2 |
| M1 白名单双源漂移 | 采纳为需求（白名单由注册表生成或 CI 一致性测试） | §1.3、A1 |
| M2 Definitions() 无生产调用方 / Definition 缺 OutputSchema | 采纳（目标态澄清 + 契约字段补齐） | §1.2、§5、A1 |
| M3 YAML bomb 防护 | 采纳 | §3.2.2 |
| M4 导入预览/发布审查界面 | 采纳 | §3.2、B2-5 交付 |
| M5 幂等并发 DB 约束 | 采纳 | §3.2.3 |
| M6 mysql-replication-health 工具不存在 | 采纳（转 backlog，同 B3 一并裁定） | §6 |
| N1-N7 次要项 | 全部采纳（N1 收窄表述 §1.2；N2 版本精确匹配 §3.3；N3 import-report 可见性 §3.4；N4 契约移 T04→B2-4，见 §5；N5 文档写 10 域 §1.4；N6 嵌套值拒绝 §2.3；N7 Agent Skills 规范事实封板前联网低风险复核一次，owner=用户） | 各节 |

## 8.2 跨迭代协调（落档）

- **A1 前置修复先行独立合入**：白名单补齐（含 OceanBase）+ builtin Definition 输出契约字段 + 一致性测试，作为独立 PR 尽早合入 main，不等本迭代整体实施；与 I008 工具网关改造（调用路径）无代码重叠（联合评审 §6 事项 4）。
- **迁移编号预分配**：本迭代主功能迁移占 **0006**（`backend/migrations/sql/`，当前非归档最大 0003；0004 归 I008、0005 归 I009；`archive/` 不动）。若实际合入顺序变化，以后合入者重排号。
- **准入条件**（联合评审 §4.3）：开放问题 1 已裁定（fail-closed）、开放问题 4 已裁定（先补工具），无遗留待裁项阻塞 B2-2/B2-4 启动。

## 参考来源

- Agent Skills 规范：https://agentskills.io/specification.md
- Anthropic Agent Skills 发布：https://www.anthropic.com/research/equipping-agents-for-the-real-world-with-agent-skills
- 官方技能仓库：https://github.com/anthropics/skills
- 社区集合：https://github.com/ComposioHQ/awesome-claude-skills 、https://github.com/VoltAgent/awesome-agent-skills 、https://awesome-skills.com 、https://bestclaudecodeskills.com/categories/databases
