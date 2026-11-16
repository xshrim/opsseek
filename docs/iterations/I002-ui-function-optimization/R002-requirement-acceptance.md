# I002-R002 验收：自举诊断故障模拟与侧栏折叠方案

**验收结论：** 故障模拟与十种侧栏预览通过；全量浏览器回归未通过。用户随后指定边界尖括号样式，正式侧栏实施及回归记录见本报告追加记录。

## 任务验收汇总

| 任务 | 结果 | 证据 |
| --- | --- | --- |
| T01 故障 API | 自动化通过 | `backend/httpapi/mock_test.go`：十种指定错误、随机结果、非法/重复/畸形编号、身份限制、日志关联和级别 |
| T02 页面触发 | 自动化通过 | `frontend/src/lib/api.test.ts` 和 `frontend/e2e/console.spec.ts`：401 不刷新、不重复请求，鼠标/键盘触发，资源子页面主标题栏提示，5 秒清除 |
| T03 十种侧栏方案 | 自动化通过，待选型 | [静态预览](../../../frontend/preview/sidebar-collapse-solutions-preview.html)、`frontend/e2e/sidebar-preview.spec.ts`：逐项折叠/展开、长用户名无重叠、全部切换、深色、390px 窄屏 |

## 执行证据（2026-11-13）

- 在 `backend/` 执行 `go test ./...`、`go vet ./...`、`go test -race ./httpapi`：通过。
- 在 `frontend/` 执行 `npm run test`：13 个测试文件、70 项测试通过；`npm run check`：零错误、零警告；`npm run build`：通过，构建目录不含侧栏预览文件。
- `npx playwright test e2e/sidebar-preview.spec.ts e2e/console.spec.ts -g 'ten sidebar|simulated fault' --workers=1`：两个新增场景运行通过；使用 Chrome 验证。
- 修改的前端文件通过 `npx prettier --check`；`git diff --check` 通过。
- 预览在 1440px 宽度生成并检查截图，使用现有全局主题令牌；静态页面不请求后端、不保存偏好。

## 全量浏览器回归限制

`npm run test:e2e -- --workers=1` 发现多项旧场景与当前 UI 不匹配后中止：2 通过、7 失败、1 中断、7 未执行，不视为全量通过。主要失败为：

- 登录测试仍期望恢复会话时发出一次健康请求，实际为零。
- 非精确的“项目”按钮选择器同时匹配导航和范围选择器。
- 权限流程仍查找已不存在的“展开权限菜单”。
- 个人中心场景仍查找“窄栏悬浮展开”单选控件。
- 深色测试等待当前流程未触发的请求。

这些流程未在本次修改；没有为通过旧断言改动无关业务页面。为运行当前总览，仅补齐原测试项目数据缺失的 `summary` 字段。

## 用户确认及遗留事项

尚未执行依赖真实数据库/监控后端的集成诊断；此功能只验证 HTTP 错误、日志和页面反馈，不能证明底层依赖真实故障。未提交、合并或推送分支。

## 后续用户指定样式

用户指定采用参考图中的尖括号样式：展开时显示指向侧栏的 `<`，折叠时显示指向内容区的 `>`；控件位于侧栏边界，边线在控件处中断。

- 正式侧栏使用 Lucide 尖括号，边界定位、用户名避让、ARIA 状态与键盘切换已实现。
- `npx playwright test e2e/console.spec.ts -g 'boundary chevrons|simulated fault' --workers=1`：2 项通过；`npm run check`、`npm run build`、Prettier 和 `git diff --check` 通过。
- 主制品 `make build` 和三种 MCP 二进制 `make mcp-build` 通过。Docker MCP 镜像构建因宿主根分区剩余空间不足失败（`no space left on device`）；未清理 Docker 缓存或用户数据。
- `docs/guides/development.md` 与 `docs/guides/deployment.md` 已更新主制品和独立 MCP 构建指令。
