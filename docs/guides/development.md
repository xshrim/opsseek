# 开发指南

所有命令从仓库根目录执行。运行方式二选一：

| 运行方式 | 用途 |
| --- | --- |
| 容器数据库 + 本机进程 | 便于开发调试，支持单独重启组件、调试器和前端热更新 |
| 容器数据库 + 容器服务 | 便于服务部署，使用镜像和 Compose 管理运行环境 |

## 1. 快速开始

### 1.1 准备环境

本机开发需要 Go、Node.js/npm；容器运行需要 Docker 和 Docker Compose。具体版本以 `backend/go.mod`、`frontend/package.json` 为准。

```bash
make env                         # .env 不存在时从 .env.example 创建，并同步根目录两个环境文件到 deploy/compose/
# 按需修改根目录 .env；不要直接编辑 deploy/compose/ 下的副本
make deps                        # 本机开发时下载 Go 和前端依赖；仅容器运行可跳过
```

### 1.2 容器数据库 + 本机进程

一键运行嵌入前端的 App：

```bash
make run                         # 同步环境、启动 PostgreSQL、准备前端资源，并从源码运行 App
# 默认访问 http://localhost:8080/opskeeper/；Ctrl+C 停止 App
make db-down                     # 停止数据库，保留数据卷
```

需要分别调试前后端时，使用以下方式替代：

```bash
make db-up                       # 先启动 PostgreSQL
make api-run                     # 终端 1：运行 API 源码
make frontend-run                # 终端 2：启动 Vite，默认访问 http://localhost:5173/opskeeper/
make worker-run                  # 另开终端：需要巡检执行时运行 Worker
make scheduler-run               # 另开终端：需要巡检调度时运行 Scheduler

# 已有数据库时，也可单独运行嵌入前端的 App；不与 api-run 同时占用同一端口
make app-run                     # 准备前端资源并运行 App，不启动数据库；前端修改后需重新运行
```

### 1.3 容器数据库 + 容器服务

默认运行嵌入前端的 App：

```bash
make up                          # 同步环境、构建 App 镜像，再启动 PostgreSQL + App
# 默认访问 http://localhost:8080/opskeeper/
make down                        # 停止 PostgreSQL 和 App，保留数据卷
```

需要拆分组件部署时，使用以下方式替代，不要同时运行占用同一端口的 App 和 API：

```bash
make db-up                       # 启动 PostgreSQL
make api-up                      # 构建 API 镜像并启动容器
make frontend-up                 # 构建独立前端镜像并启动容器，默认访问 http://localhost:5173/opskeeper/
make worker-up                   # 需要巡检执行时，构建并启动 Worker 容器
make scheduler-up                # 需要巡检调度时，构建并启动 Scheduler 容器

docker compose -f deploy/compose/docker-compose.yml --env-file .env --profile svc stop api worker scheduler frontend postgres  # 停止拆分服务及数据库，保留数据
make clean                       # 仅需重置环境时执行：删除 Compose 容器、网络和数据卷，数据库数据不可恢复
```

API、Worker、Scheduler、Frontend 属于 `svc` profile，App 不属于任何 profile。Compose 只运行镜像，不执行构建；上述启动入口通过 Makefile 前置目标完成构建。

## 2. 检查与构建

### 2.1 测试与检查

```bash
make test                        # 集中运行 Go 和前端单元测试
make backend-test                # 只运行 Go 单元测试
make frontend-test               # 只运行前端单元测试
make backend-embedded-test       # 构建前端资源并验证嵌入式 Web UI 和 HTTP API
make backend-integration-test    # 使用 .env 中的测试数据库配置执行集成测试，需先准备测试数据库

make lint                        # 集中执行 Go、前端、部署脚本和 Helm 静态检查，需安装 Helm
make backend-lint                # 只运行 go vet
make frontend-lint               # 只运行 svelte-check
make format                      # 格式化 Go 和前端源码
make format-check                # 只检查 Go 和前端格式
make quality                     # 完整门禁：格式、静态检查、单元测试、嵌入式测试和二进制构建，不构建镜像
```

### 2.2 二进制与前端制品构建

**单项构建**

每个 Makefile 构建都会注入 `VERSION`、`COMMIT`、`BUILD_TIME`。App 自动准备嵌入式前端资源：

```bash
# 只构建一个主程序或前端静态制品
make api-build
make worker-build
make scheduler-build
make app-build
make frontend-build

# 只构建一个 MCP 二进制
make docker-mcp-build
make host-mcp-build
make kubernetes-mcp-build
```

**分组与集中构建**

```bash
# 分组构建 API、App、Worker、Scheduler 四个二进制
make backend-build
# 分组构建 Docker、Host、Kubernetes MCP 二进制
make mcp-build

# 集中构建 API、Worker、Scheduler、Frontend、App，不含 MCP
make build
```

Go 二进制位于 `backend/bin/`；嵌入前端的 App 和不含前端的 API 是两个制品。

### 2.3 容器镜像构建

**单独构建**

```bash
make api-image                   # 构建 opskeeper-api:local，不含前端
make worker-image                # 构建 opskeeper-worker:local
make scheduler-image             # 构建 opskeeper-scheduler:local
make frontend-image              # 构建 opskeeper-frontend:local，Nginx 静态前端
make app-image                   # 构建 opskeeper-app:local，嵌入前端

# 只构建一个 MCP 镜像
make docker-mcp-image             # 构建 opskeeper-docker-mcp:local
make host-mcp-image               # 构建 opskeeper-host-mcp:local
make kubernetes-mcp-image         # 构建 opskeeper-kubernetes-mcp:local
```

**集中构建**

```bash
make image                       # 集中构建以上五个服务镜像，不启动容器
make mcp-image                   # 集中构建三个 MCP 镜像，不启动容器

# 覆盖仓库前缀和标签；单项 *-image 目标同样支持这些参数，不自动推送镜像
make image IMAGE_REPOSITORY=registry.example.com/opskeeper IMAGE_TAG=v0.1.0
```

镜像名称为 `<IMAGE_REPOSITORY>-<服务>:<IMAGE_TAG>`，默认前缀 `opskeeper`、标签 `local`。默认使用 `deploy/Dockerfile`：`*-image` 先在本机构建对应制品，再复制打包；后端自动编译为静态 Linux 二进制，本地需安装 Go、Node 及前端依赖。原容器内源码编译链路保留在 `deploy/Dockerfile.prd`，可用 `make image DOCKERFILE=deploy/Dockerfile.prd`。跨架构时需让 `GOARCH` 与 Docker 目标平台一致。Makefile 不自动读取 `.env` 中的镜像变量，使用非默认值时须通过命令参数或导出的环境变量传入，并与 Compose 配置保持一致。

## 3. 配置与日常操作

### 3.1 配置、迁移与管理员

以根目录 [.env.example](../../.env.example) 为配置项参考，只在 `.env` 中保留需要特定配置的值。Compose 使用 `postgres:5432` 和 `http://otel-collector:4318` 访问容器服务；本机进程使用 `.env` 中的地址。

API/App 启动时连接数据库、获取迁移锁并应用待执行迁移，失败则不监听 HTTP。Worker、Scheduler 不执行迁移。首次访问登录页时初始化管理员密码，无需执行独立的 `migrate` 或 `admin` 命令。

### 3.2 样例数据

```bash
# API/App 已启动并完成迁移后执行；只允许连接本地 development 数据库
make sample-seed                 # 写入演示数据
make sample-clean                # 只删除带 sample 标记的演示数据
```

### 3.3 数据库与可观察性

```bash
make db-up                       # 启动 PostgreSQL
make db-logs                     # 查看数据库日志
make db-down                     # 停止数据库，保留数据卷
make db-clean                    # 删除数据库容器和数据卷，执行前确认备份

make obs-up                      # 启动 Jaeger、Collector、Prometheus、Loki、Grafana
make obs-logs                    # 查看监控组件日志
make obs-down                    # 停止监控组件，保留数据卷
make obs-clean                   # 删除监控组件容器和数据卷

make infra-up                    # 同时启动数据库和监控组件
make infra-logs                  # 查看数据库和监控组件日志
make infra-down                  # 同时停止数据库和监控组件，保留数据卷
make infra-clean                 # 删除数据库、监控组件及其数据卷
```

Trace 和 Metrics 经 OTLP/HTTP 发到 Collector，分别进入 Jaeger 和 Prometheus，由 Grafana 展示。日志仍写标准输出，当前未接入 Loki 采集。容器启动不会自动启用监控 profile，需要单独启动监控组件。

## 4. 命令速查

下表中 Make 目标均以 `make <目标>` 执行。

| 类别 | 范围 | 目标 |
| --- | --- | --- |
| 环境 | 初始化与依赖 | `env`、`deps` |
| 本机运行 | 组合入口 / 单项 | `run` / `api-run`、`worker-run`、`scheduler-run`、`frontend-run`、`app-run` |
| 容器运行 | 组合入口 | `up`、`down`、`clean` |
| 容器运行 | 单项启动（先构建） | `api-up`、`worker-up`、`scheduler-up`、`frontend-up`、`app-up` |
| 容器运行 | 单项停止 / 清理 / 日志 | `api-*`、`worker-*`、`scheduler-*`、`frontend-*`、`app-*`，后缀为 `down`、`clean`、`logs`；单项 `clean` 仅停止并删除对应容器，不删除共享数据卷或镜像 |
| 二进制构建 | 主程序单项 | `api-build`、`worker-build`、`scheduler-build`、`app-build` |
| 二进制构建 | 主程序分组 / 集中 | `backend-build` / `build`（含前端，不含 MCP） |
| 二进制构建 | MCP 单项 / 集中 | `docker-mcp-build`、`host-mcp-build`、`kubernetes-mcp-build` / `mcp-build` |
| 前端静态制品 | 单项 / 嵌入资源 | `frontend-build` / `webui-assets` |
| 容器构建 | 单项 | `api-image`、`worker-image`、`scheduler-image`、`frontend-image`、`app-image` |
| 容器构建 | 集中（五个镜像） | `image` |
| 容器构建 | MCP 单项 / 集中 | `docker-mcp-image`、`host-mcp-image`、`kubernetes-mcp-image` / `mcp-image` |
| 测试 | 集中 / 单项 | `test` / `backend-test`、`frontend-test`、`backend-embedded-test`、`backend-integration-test` |
| 检查 | 集中 / 单项 | `lint` / `backend-lint`、`frontend-lint`、`format`、`format-check` |
| 质量门禁 | 检查、测试、构建 | `quality` |
| 样例数据 | 写入 / 清理 | `sample-seed`、`sample-clean` |
| 数据库 | 启动 / 停止 / 清理 / 日志 | `db-up`、`db-down`、`db-clean`、`db-logs` |
| 可观察性 | 启动 / 停止 / 清理 / 日志 | `obs-up`、`obs-down`、`obs-clean`、`obs-logs` |
| 数据库 + 可观察性 | 启动 / 停止 / 清理 / 日志 | `infra-up`、`infra-down`、`infra-clean`、`infra-logs` |

发布和回滚见[部署指南](deployment.md)，领域设计见 [design/](../design/)。
