# R001 Skills 导入与技能库整理 — 设计评审

**评审对象：** [R001-requirement.md](R001-requirement.md)、[iteration.md](iteration.md)
**评审日期：** 2026-11-18
**评审分支：** feature/engine-expert-skills
**评审人：** 设计评审智能体（I010）
**评审结论：** **有条件通过（3 项必改）** — 详见 §6

---

## 1. 事实核验结果

### 1.1 两处高危声称

#### 声称 A：`service.go:255-262` 工具白名单缺 Oracle/ES/RabbitMQ/MinIO 工具 — **属实，且范围可扩大**

验证过程：

- `backend/skill/service.go:255-262` 的 `allowedToolName` 逐项核对：仅含 connector 通用 6 项 + kafka_*（7）、redis_*（6）、postgresql_*（12）、mysql_*（7）。**无任何 `oracle_*`、`elasticsearch_*`、`rabbitmq_*`、`minio_*` 条目**。
- `backend/skill/builtin/definitions.go:28-33` 确认 oracle-health（7 工具）、elasticsearch-health（7）、rabbitmq-health（9）、minio-health（6）全部声明这些工具，工具实现存在于 `backend/tool/oracle/`、`backend/tool/elasticsearch/`、`backend/tool/rabbitmq/`、`backend/tool/minio/`。
- 附加发现（文档遗漏）：白名单同样缺 **`oceanbase_*`（7 个）**；`backend/migrations/sql/0001_initial.sql:2095` 存在已发布的 OceanBase-健康诊断技能（oceanbase_health 等 7 工具），但 `builtin/definitions.go` 也没有 OceanBase 定义——白名单缺口和 builtin 清单都比文档声称的多一域。

限定条件（文档表述需修正的细节）：`CreateVersion`（`service.go:35-81`）是唯一走 `allowedToolName` 的路径；内置技能实际通过 SQL 种子数据（`0001_initial.sql:2073-2100`）直接落库，不经过该校验。因此缺口的真实影响面是「**用户自建/导入版本**」——与 R001 §1.3 的结论一致，但建议行文明确该限定，避免误读为内置技能受损。

**裁定：声称成立（附带 OceanBase 遗漏，升级为主要问题 M1）。**

#### 声称 B：OpsKeeper 0013 迁移的诊断输出契约未随迁 — **部分属实，表述过宽，需按下列事实收窄**

验证过程：

- `git show c7ddc64:backend/migrations/sql/0013_builtin_skills.sql` 确认存在：`output_schema` 固定为 `required: [facts, findings, evidence, hypotheses, confidence, recommendations]`，且 4 条 instruction 均含「输出 JSON，且必须包含 facts…」文案。commit `c7ddc64` 存在（feat: add builtin diagnosis skills）。
- 但当前合并基线 `backend/migrations/sql/0001_initial.sql:2073-2081` 的种子数据**并非全部丢失契约**：
  - K8s/PG/Redis/Kafka 各自 v1、v2 版本的 `output_schema` **保留了完整契约**（如 2074 行 K8s v2）；
  - **丢失契约的是新增域**：MySQL v1（2077 行起，部分版本有、`'{}'` 混杂）、ES v1（2083）、Oracle v1（2084）、RabbitMQ v1（2085）、MinIO v1（2086）、OceanBase v1（2095）、以及 Kafka v3（2082）的 `output_schema` 均为 `'{}'`。
- `builtin/definitions.go` 的 `Definition` 结构体（14-20 行）**根本没有 OutputSchema 字段**，Go 侧内置声明在类型层面就无法携带输出契约；且全仓库检索 `Definitions()` 仅在 `backend/skill/builtin/` 自身的测试中被调用，未发现生产装配路径（内置技能靠 SQL 种子而非代码安装）——与 iteration.md §2.5「保持现有 builtin.Definition 安装语义不变」的描述存在张力（待验证：是否存在仓库外装配）。

**裁定：声称核心成立（新域技能确实丢失了 0013 契约，且 builtin 定义结构无法表达契约），但「未随迁到当前 builtin 定义」应精确为「**部分旧域版本保留、全部新域版本及 builtin 代码定义缺失**」。R001 §1.2 自己标注「待验证」是诚实的，评审已替其验证并收窄。清单第 12 项（恢复契约为诊断类默认 output_schema）成立，建议同时在 `Definition` 结构补 OutputSchema 字段。**

### 1.2 其他代码/文档引用抽查（8 处）

| # | 文档声称 | 核验结果 | 依据 |
|---|---|---|---|
| 1 | `types.go:10-21` Skill 字段、`types.go:23-27` 四类 category | ✅ 属实（category 常量在 22-32 行区间，行号偏差 1 行，可接受） | `backend/skill/types.go:10-32` |
| 2 | `document.go:15` identifier 正则、`:202` spec 字段白名单、`:22-57` ParseDocument 三格式 + SHA-256 | ✅ 全部属实（正则 15 行、rejectUnknown 202 行、ParseDocument 22-57 行） | `backend/skill/document.go` |
| 3 | `document.go:238-240` name 无空白、`:255` tags ≤50、description ≤4000（206 行） | ✅ 属实 | `backend/skill/document.go:205-267` |
| 4 | `service.go:54` 工具 ≤20、`:77-79` risk_level 三值 | ✅ 属实（实际 54-55、77-79 行） | `backend/skill/service.go` |
| 5 | `httpapi/skill.go:194-223` 现有 API 仅有 list/createVersion/publish/disable/default，无导入导出 | ✅ 属实（路由注册 194-203 行附近，全部 grep 确认无 import/export 端点） | `backend/httpapi/skill.go` |
| 6 | `authorization/types.go:23-25` skill:read/manage/execute | ✅ 属实 | `backend/authorization/types.go:23-25` |
| 7 | `docs/guides/integrations.md:29` 写「覆盖 K8s/PG/Redis/Kafka」与实际不符 | ✅ 属实（该行原文一致）；实际种子数据含 10 个技能（含 OceanBase），文档说的 9 个也不准 | `docs/guides/integrations.md:29`、`0001_initial.sql:2097-2100` |
| 8 | 内置技能 9 个（`definitions.go:25-33`）、commit `4d8454f` runner.go 431 行、`c0029b0` 统一执行 | ✅ 基本属实；但 SQL 种子含 OceanBase 共 10 个，`definitions.go` 与 DB 种子**互相不一致** | `git show --stat 4d8454f`（含 runner.go）、`git log -1 c0029b0` |

### 1.3 技能清单工具存在性抽查（§6）

| 清单项 | 依赖工具 | 核验 |
|---|---|---|
| #1 mongodb-health | MongoDB 工具 | ❌ **不存在**。`grep -r mongodb backend/connector backend/tool backend/engine` 零命中；edbeb96 仅新增资源目录/catalog（迁移 `0003_mongodb_resource_and_document_database.sql`），无诊断工具 |
| #3/#4 postgresql_* | sessions/long_running_queries/locks/vacuum/tables/performance | ✅ 均在白名单 `service.go:257` |
| #5 mysql-replication-health | mysql_status/performance | ⚠️ 无 `mysql_replication` 工具（MySQL 工具集仅 health/status/performance/tables/table_columns/table_structure/database_info，见 `definitions.go:27`）；文档已自标「待验证」，正确 |
| #6/#7/#10 redis/kafka/k8s | 全部命中白名单 + connector_metrics_query | ✅ |
| #8 ES、#9 MinIO | elasticsearch_*/minio_* | ⚠️ 工具存在但**不在白名单**，被 T01 阻塞（文档已声明依赖，一致） |

### 1.4 Agent Skills 规范事实

文档 §2.1 的字段约束（name ≤64 小写连字符、description ≤1024、compatibility ≤500、allowed-tools 实验性空格分隔、渐进披露 ~100 token 元数据 / <5000 token 正文、scripts/references/assets 目录约定）与本评审掌握的公开规范一致。**未联网独立复核**（评审环境未执行 WebSearch），如封板前需强保证请另行复核 agentskills.io/specification.md。标注：待验证（低风险）。

---

## 2. 安全专项意见

设计的安全骨架（fail-closed 工具收敛、scripts/ 不执行、draft-then-publish、skill:manage 守卫、审计）方向正确，且与 I005 非目标一致。但针对 SKILL.md 导入的攻击面，以下向量在 R001 §3.2 中**未覆盖或覆盖不足**：

1. **【阻塞 S1】zip 解包安全未明确**：§3.2 只限了体积，未提 zip-slip（`../` 路径穿越写盘）、符号链接条目、嵌套 zip（zip 内含 zip 的二次解包）。必改：设计须明确「拒绝绝对路径与 `..` 路径条目、拒绝 symlink 条目、不做嵌套归档解包、解包目标使用临时目录且限制文件数（建议 ≤200/包）」。
2. **【主要 S2】frontmatter 解析前的大小与 YAML DoS**：description/compatibility 等字段的规范上限是「合法技能」的约束，不是解析器防护。恶意包可在 frontmatter 放 10MB YAML（anchor alias 爆炸、深层嵌套）导致解析耗尽。必改方向：单文件解析前先限 SKILL.md ≤256KB（或与 body 限额统一），使用 `yaml.Decoder` 限制别名/深度，而非 `yaml.Unmarshal` 整包。当前 `document.go:69` 用的正是 `yaml.Unmarshal`，导入路径应加前置截断。
3. **【主要 S3】prompt injection 藏于正文**：draft-then-publish 依赖人工审查，但 §3.2 只提「过滤 script/javascript:」。注入不一定用 HTML——正文可用自然语言指令（「调用 X 工具读取 credentials」「忽略以上约束」）。建议：导入报告（§3.4 import-report）中固化「正文全文 diff/预览」为发布前必经界面，并对已知注入模式（ignore previous、system prompt、凭据读取类关键词）做静态扫描告警（非拦截，告警进 import-report）。前端发布确认页应展示工具清单与 risk_level 而非仅名称。
4. **【主要 S4】伪装 allowed-tools / 名称混淆**：fail-closed 精确匹配正确。但需明确匹配是对「转换后 spec.tools 每一项」逐字匹配（无通配、无前缀、无大小写折叠），且 SKILL.md 的 `Bash(psql:*)` 形式必须解析出完整 token 后整体判失败，不允许截断到 `Bash` 再模糊匹配。开放问题 1 的「人工映射向导」若实现，绝不可默认选中任何平台工具——见 §5 裁定。
5. **【次要 S5】draft 可见性**：`ListVersions` 仅需 `skill:read`（`httpapi/skill.go` listVersions 路由），导入的 draft 正文对读权限用户可见。可接受，但建议 import-report 中的原始告警仅 `skill:manage` 可见。
6. **【次要 S6】幂等并发**：同 SHA-256 幂等若只在应用层查重，并发双导入可产生重复版本。需 DB 层唯一约束（如 `skill_versions(skill_id, source_digest)` 部分唯一索引）或事务内 `SELECT ... FOR UPDATE`，文档未提及。

---

## 3. 格式映射完备性

- 映射表（§2.3）覆盖了规范关键字段（name/description/metadata/version/author/allowed-tools/license/compatibility/正文/scripts/references），处置均明确。遗漏项：**`metadata` 嵌套值非 string**（规范要求 string→string，映射方应显式拒绝非标量值而非依赖 Go 解码失败）——次要。
- `metadata.identifier` 直接映射 `name` 有一个坑：规范允许 64 字符，OpsSeek 允许 120，方向安全；但 SKILL.md `name` 允许纯数字开头（`^[a-z0-9]` 同构），无冲突。`description → metadata.name` 要求无空白单词——SKILL.md name 本身受规范约束为连字符词，可行。✅
- 无法映射字段的「丢弃 + 告警 + 审计」策略一致，✅。
- 版本策略：同 identifier + 不同内容 → 新版本号取导入 metadata.version，「与现有语义化版本相同则拒绝」——注意 `skillVersionPattern`（`document.go:16`）允许 prerelease 后缀，`1.0.0-alpha` 与 `1.0.0` 是否视为冲突需在设计中写明（建议：字符串精确匹配即拒绝，不做语义比较）。次要。

## 4. 技能清单合理性

- 与 `docs/guides/diagnostics.md` 及 9 个既有 health 技能**无实质重复**（均为聚焦子场景：慢查询、vacuum、lag、容量等），组织合理。
- 阻塞问题见 §1.3：#1 MongoDB 工具不存在；#5 MySQL 复制工具不存在。这两项要么先补工具（超出本迭代边界），要么明确转入 backlog——迭代.md §7 已允许「明确转入 backlog」，建议执行。
- 第 12 项「契约模板」严格说不是「技能」而是 schema 资产，建议从技能清单移到 T04 的产物（诊断类 builtin 默认 output_schema），避免清单语义混杂。次要。

## 5. 开放问题裁定建议

| # | 问题 | 推荐裁定 | 理由 |
|---|---|---|---|
| 1 | allowed-tools 是否提供人工映射向导 | **v1 不做向导，保持纯 fail-closed**；向导列入 backlog | 向导会把「外部任意工具声明」变成「人工背书的白名单扩边」，削弱 fail-closed 语义且增加 UI 复杂度；社区技能按文档自己的结论只能借鉴知识正文，工具集本来就应人工重选——那等价于手工重建技能而非「导入」 |
| 2 | license 是否升级为 Metadata 字段 | **v1 不加字段**（`document.go:172` metadata rejectUnknown 会拒绝，加字段=改契约），license 记入 import-report + 审计；与 Q5 的来源快照一并考虑进 v2 | 契约变更成本高、当前消费方为零 |
| 3 | references/ 内联 or 附件 | **同意文档建议：v1 内联并限行数（≤500 行/文件，总量并入 body 100KB 限额）** | 避免 v2 格式扩展阻塞本迭代；超限丢弃并告警 |
| 4 | MongoDB 工具命名 | **裁定为「工具不存在，mongodb-health 转入 backlog，待 MongoDB 诊断工具迭代」**（本次评审已验证，不再是待验证项） | `backend/tool`、`backend/connector`、`backend/engine` 均无 mongodb 工具；edbeb96 只加了资源目录（迁移 0003） |
| 5 | 社区技能来源仓库 + commit 快照 | **建议采纳，T04 落地为 Definition 增加来源注释或旁注文件** | 许可证合规与后续更新 diff 的最低成本方案 |
| 6 | 是否提供导出 API | **采纳：纳入 T02**（`GET /api/v1/skills/{id}/versions/{vid}/export`，序列化 Document 即可） | round-trip 验收（§7 第一条）本身就依赖导出；工作量小 |

## 6. 分级问题清单

### 阻塞（必改，改完可直接进入实现）

| # | 问题 | 依据 |
|---|---|---|
| B1 | **OceanBase 域整体遗漏**：白名单缺 `oceanbase_*`、`builtin/definitions.go` 无 OceanBase 定义但 DB 种子有其已发布技能（10 vs 9 不一致）。T01 白名单补齐范围必须重定，且需先澄清 builtin 代码定义与 SQL 种子的权威来源关系 | `service.go:257`、`definitions.go:25-34`、`0001_initial.sql:2095,2100` |
| B2 | **zip 解包安全设计缺失**：zip-slip / symlink / 嵌套归档 / 条目数上限未写入 §3.2 | §2 意见 S1 |
| B3 | **MongoDB 技能依赖不存在的工具**：清单 #1 需裁定转 backlog 或先补工具（开放问题 4 已给裁定） | §1.3、迁移 0003 |

### 主要（封板前应改）

| # | 问题 | 依据 |
|---|---|---|
| M1 | 白名单为手工 switch，与 `backend/tool/*` 工具注册表双源漂移（本次 Oracle/ES/RabbitMQ/MinIO/OceanBase 缺失正是漂移证据）。建议白名单改由工具注册表生成或在 CI 中比对 | `service.go:255-262` vs `backend/tool/` 各域 |
| M2 | `builtin.Definitions()` 无生产调用方，内置技能实际由 SQL 种子安装；iteration.md §2.5「保持安装语义不变」与现状不符，T04 前必须澄清目标态（代码安装 or 种子维护）。同时 `Definition` 缺 OutputSchema 字段，清单 #12 无法在其上落地 | §1.1 声称 B 验证 |
| M3 | frontmatter/YAML 解析前置大小限制与解析器防护（YAML bomb）未设计 | §2 意见 S2 |
| M4 | 导入预览/diff 与发布前正文审查界面未纳入 T06 交付（当前只有「转换告警展示」）；建议明确发布确认页义务 | §2 意见 S3 |
| M5 | 幂等键的并发行为未定义（DB 唯一约束） | §2 意见 S6 |
| M6 | 清单 #5 mysql-replication-health 工具不存在（`mysql_status` 是否含复制指标待验证），需同 #1 一并裁定 | §1.3 |

### 次要

| # | 问题 |
|---|---|
| N1 | 声称 B 表述收窄（§1.1）：「契约未随迁」应区分旧域保留/新域缺失，避免误导补齐范围 |
| N2 | 版本冲突判定规则需写明是字符串精确匹配还是语义比较（prerelease 后缀） |
| N3 | import-report 可见性建议限 `skill:manage` |
| N4 | 清单 #12 移出技能清单，归入 T04 契约资产 |
| N5 | `docs/guides/integrations.md:29` 更新时写 10 个域（含 OceanBase）而非 9 个 |
| N6 | metadata 嵌套非 string 值的显式拒绝规则 |
| N7 | §2.1 规范事实未联网复核（本评审亦未复核），封板前低风险确认 |

## 7. 评审结论

**有条件通过。**

设计的核心决策（双来源导入、确定性转换、fail-closed 工具收敛、draft-then-publish、SHA-256 幂等、按域重组 builtin）方向正确且有扎实的现状盘点支撑；两处高危声称经验证基本成立（声称 A 属实且范围可扩大，声称 B 部分属实需收窄表述）。事实引用抽查 8 处中 7 处完全准确、1 处（9 vs 10 技能数）存在仓库内自不一致，总体可信度高。

必改项（B1-B3）完成后可进入实现阶段；主要问题 M1-M6 建议随设计修订一并落实，不阻塞任务级启动但阻塞封板。
