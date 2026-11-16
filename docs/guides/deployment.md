# 部署指南

## 1. 制品

使用根目录 Makefile 构建，Compose 不负责 build：

默认使用 `deploy/Dockerfile`：先在本地构建制品，再复制到精简运行镜像；Docker 内不安装源码依赖、不执行编译，也不携带 Go/Node 工具链。后端使用 Alpine，仅安装 CA 证书，以非 root 用户运行；Git 仓库、Bundle 与 SSH 均由 Go 库处理，运行镜像不包含 Git 或 OpenSSH。前端使用 Nginx Alpine Slim，提供静态文件和 API 代理。本地需准备 Go、Node 和前端依赖（首次执行 `make deps`）。

代码仓库支持 HTTPS 用户名/访问令牌，以及 SSH 密码、默认私钥或 SSH Agent；SSH 连接必须提供可信 `known_hosts`。容器使用 SSH 仓库时，须只读挂载对应凭据至 `/home/nonroot/.ssh`，或显式挂载 Agent socket。不会执行 `GIT_SSH_COMMAND`、Git hooks、外部凭据助手或 SSH `ProxyCommand`；需要这些外部程序的部署应改用可直连地址。Bundle 支持 SHA-1 格式的 v2/v3 全量与增量上传；SHA-256 或不支持的 Bundle capability 会明确报错。诊断正则检索使用 Go 正则语法，不支持反向引用和环视。

```bash
# 运行发布前检查
make quality
# 构建 API、Worker、Scheduler、Frontend、App 五个服务镜像
make image IMAGE_REPOSITORY=registry.example.com/opsseek IMAGE_TAG=v0.1.0
# 保留的生产多阶段链路：在容器内完成依赖安装、源码编译和打包
make image DOCKERFILE=deploy/Dockerfile.prd IMAGE_REPOSITORY=registry.example.com/opsseek IMAGE_TAG=v0.1.0
```

`*-image` 和 `mcp-image` 均支持上述 `DOCKERFILE` 选择。快速链路自动使用 `GOOS=linux`、`CGO_ENABLED=0`；默认架构为宿主机架构，跨架构时必须让 `GOARCH` 与 Docker 的目标平台一致。直接执行 `docker build -f deploy/Dockerfile` 前，需自行准备对应的 Linux 二进制及 `frontend/dist`。快速链路仅传入制品和 Nginx 配置，生产链路继续使用根目录 `.dockerignore` 排除本地制品。

生成五个同版本镜像：

```text
opsseek-api        # 非嵌入前端的 API
opsseek-worker     # 巡检任务执行器
opsseek-scheduler  # 巡检调度器
opsseek-frontend   # Nginx 静态前端
opsseek-app        # 嵌入前端的单进程服务
```

发布应记录每个镜像 digest，不使用 `latest`。构建阶段不读取运行时 Secret。

MCP 制品与服务制品分开构建，不会被 `make build` 或 `make image` 隐式带入：

```bash
# 构建 Docker、Host、Kubernetes MCP 二进制
make mcp-build
# 只构建一个 MCP 镜像
make docker-mcp-image IMAGE_REPOSITORY=registry.example.com/opsseek IMAGE_TAG=v0.1.0
# 构建三个 MCP 镜像
make mcp-image IMAGE_REPOSITORY=registry.example.com/opsseek IMAGE_TAG=v0.1.0
```

对应镜像为 `opsseek-docker-mcp`、`opsseek-host-mcp`、`opsseek-kubernetes-mcp`。它们默认分别监听 `8811`、`8813`、`8812`，运行时必须单独提供 Bearer Token 和所需访问能力。Docker MCP 需要显式挂载可访问的 Docker socket；Host MCP 在容器中只会观察容器自身，除非按部署边界额外挂载主机资源；Kubernetes MCP 需要提供集群网络和凭据。不要把这些高权限挂载或令牌写入镜像。

## 2. Compose

准备根目录 `.env`，然后启动默认单进程模式：

```bash
make env
make app-image
# 直接使用 Compose 时，为非 root 服务加入宿主机 Docker socket 所属组
export DOCKER_SOCKET_GID=$(stat -c '%g' /var/run/docker.sock)
docker compose -f deploy/compose/docker-compose.yml --env-file .env \
  up -d --wait --wait-timeout 180 postgres app
```

拆分服务模式：

```bash
make api-up
make worker-up
make scheduler-up
make frontend-up
```

`api`、`worker`、`scheduler`、`frontend` 属于 `svc` profile；`app` 为默认服务。服务不使用 `condition`，程序自身负责依赖重试。Compose 文件只引用镜像，不包含 `build`。

App/API 将宿主机 `/var/run/docker.sock` 挂载到容器同一路径，Docker Direct 资源使用 `unix:///var/run/docker.sock` 即可访问宿主机容器。`make up`、`make app-up` 和 `make api-up` 自动识别 socket 的所属组，无需修改镜像或使用 root 用户。镜像构建不读取或挂载 socket，宿主机没有 socket 也能构建；运行上述服务则需要存在该 socket，缺失时明确报错，不自动创建同名目录。无本地 Docker 的部署需移除此挂载，并配置远程 Docker 端点。

Docker socket 访问权限基本等同于宿主机管理员权限，仅向可信服务开放；它不只是只读监控接口。

停止和清理：

```bash
make down       # 停止 PostgreSQL 和 App
make clean      # 删除 Compose 数据卷
```

## 3. Helm

生产部署使用 `deploy/helm/opsseek`。Chart 的 API 工作负载使用嵌入前端的 `opsseek-app` 镜像，以便 Ingress 直接提供页面；Worker 和 Scheduler 使用独立镜像。

创建运行时 Secret（示例值不要提交）：

```bash
kubectl -n opsseek create secret generic opsseek-runtime \
  --from-literal=OPSK_DB_HOST='<database-host:5432>' \
  --from-literal=OPSK_DB_NAME='opsseek' \
  --from-literal=OPSK_DB_USER='<application-user>' \
  --from-literal=OPSK_DB_PASSWORD='<application-password>' \
  --from-literal=OPSK_CREDENTIAL_KEY='<32-byte-or-base64-key>'
```

准备 values：

```yaml
image:
  repository: registry.example.com/opsseek
  digest: sha256:<digest>
basePath: /opsseek
existingSecret: opsseek-runtime
ingress:
  enabled: true
  host: ops.example.com
  tlsSecretName: opsseek-tls
```

部署和验证：

```bash
helm lint deploy/helm/opsseek
helm upgrade --install opsseek deploy/helm/opsseek \
  --namespace opsseek --create-namespace --atomic --timeout 15m \
  -f production-values.yaml
kubectl -n opsseek wait --for=condition=available \
  deployment/opsseek-opsseek-api --timeout=5m
curl --fail --silent https://ops.example.com/opsseek/health/ready
```

## 4. 迁移与权限

本次更名不兼容旧部署：数据库和账号默认改为 `opsseek`，访问前缀改为 `/opsseek`，镜像、容器、二进制和 Helm Chart 均使用新名称；环境变量名仍使用 `OPSK_`。请使用新数据库初始化，不要将旧迁移记录直接交给新版本；开发环境默认凭据加密密钥也已更换，旧密文不能直接复用。更名前的数据应单独备份，需要保留的业务配置须重新导入并填写凭据。

API/App 在监听端口前自动迁移，并通过 PostgreSQL advisory lock 防止并发执行。运行账号只需要本平台业务数据库的迁移权限，不需要 PostgreSQL 实例级超级用户权限，也不需要独立 Migration Job。

`backend/migrations/sql/0001_initial.sql` 直接定义当前完整结构和内置数据，新库仅执行此文件；历史 SQL 保留在 `archive/`。已有数据库会校验原迁移历史、补齐归档链路中的缺失迁移，再事务性合并为初始化基线记录，不重建表、不清空业务数据。不要手工删除迁移历史来强行执行初始化 SQL。

迁移采用前向兼容的 Expand/Contract 方式。发布顺序为：

1. 发布 API/App，等待迁移和 readiness 成功。
2. 发布 Worker 和 Scheduler。
3. 验证登录、只读资源、诊断、巡检和通知。

迁移失败时停止发布；不要在生产环境执行反向迁移。

应用关联数据库时，资源中的数据库名是默认连接库，关联中的数据库名是本次应用诊断的业务目标。Direct 和 Agent 均使用资源保存的诊断凭据连接业务目标，不改写资源默认值；关联中的应用用户名不是登录凭据。无需在关联中重复保存密码：诊断账号需具备目标库的连接、统计及所需对象的只读权限，失败时不会静默退回默认库。同一资源关联不同业务目标时，先选定一个应用再诊断。数据库统计仍以工具实际支持的数据库或实例范围为准，不等同于应用独占统计。

## 5. 回滚

1. 保留当前和目标镜像 digest、Chart 版本及数据库备份校验和。
2. 应用失败时回滚到与当前 Schema 兼容的旧镜像。
3. Schema 不兼容时前滚修复，不执行 `down` migration。
4. 回滚后检查 readiness、任务租约、通知幂等和错误率。

完整故障处置见 [runbooks/](../runbooks/)。
