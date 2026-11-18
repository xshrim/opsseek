# R001 验收记录：技能前置修复与导入功能

**所属迭代：** [I010](iteration.md) | **验收日期：** 2026-11-18 | **状态：** 实施完成（待运行环境验收）

## 交付范围与证据

| 项 | 交付 | 证据 |
| --- | --- | --- |
| A1 白名单单一来源 | 白名单由 backend/tool 各域 ListTools() 动态生成（含 Oracle/ES/RabbitMQ/MinIO/OceanBase 五域全量工具+7 个遗留 connector_*），删除 service.go 手工 switch，双源漂移结构性消除；顺手修复 tool/host darwin 编译（statfs_darwin.go） | commit c1229d1 `backend/skill/whitelist.go(+test)` |
| A1 输出契约 | builtin Definition 增 OutputSchema；DiagnosisOutputSchema 与 OpsKeeper 0013 迁移逐字一致（facts/findings/evidence/hypotheses/confidence/recommendations）；OceanBase 定义补齐对齐种子 10 域 | c1229d1 `builtin/definitions.go(+test 9→10)` |
| 迁移 0006 | skill_versions 增 source_type/source_digest + (skill_id, source_digest) 部分唯一索引；MySQL/Kafka v3/ES/Oracle/RabbitMQ/MinIO/OceanBase 种子补 0013 诊断输出契约 | commit ab52542 `0006_skill_import.sql` |
| 导入 API | POST /api/v1/skills/import（skill:manage、skill.import 审计、JSON base64）；双来源：OpsSeek 导出文档（ParseDocument 复用）+ Agent Skills zip（SKILL.md 映射：name→identifier、allowed-tools 收敛 AllowedTools() fail-closed、scripts/ 丢弃） | ab52542 `backend/skill/import.go`、`httpapi/skill.go` |
| 安全防护 | 纯内存解包（不落盘）；拒 zip-slip/绝对路径/反斜杠/symlink/嵌套归档；条目≤200、包≤10MB、解包≤50MB、压缩比≤100；SKILL.md≤256KB、frontmatter≤64KB；受限 YAML（拒 alias、深度≤100）；正文清洗+注入模式告警 | `import.go` + `import_test.go` 12 组表驱动 |
| 幂等与版本 | 规范化文档 SHA-256 幂等（重复导入返回既有版本）；同 identifier 生成新版本；导入一律 draft 人工发布 | `store.go` ImportVersion 事务内幂等 |
| 前端 | 技能页导入向导（粘贴/上传 zip、逐项结果与告警、发布前全文预览确认） | ab52542 `SkillRegistryPage.svelte`、`api.ts` |

## 验证结果

- `go test ./skill/... ./httpapi/... ./migrations/...`：192 passed（当时点）；后端全量 877 passed（详见 I008 验收记录）。
- 前端 `npm run check` 0 errors、`npm run build` ✓、`npm test` 105 passed。

## 未尽事项

1. 导出 API（B2-1 子项）未实现——本次指令范围未含，待补。
2. 集成测试未覆盖真 PostgreSQL 路径。
3. 首批技能清单中 MongoDB 诊断技能依赖「先补 MongoDB 诊断工具」前置子任务（A2 子任务），未在本次实施。
