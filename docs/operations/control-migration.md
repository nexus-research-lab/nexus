# Nexus Control 部署与迁移

服务端 Web 账号、订阅套餐与成员额度由独立的 `nexus-control` 管理。Desktop 本地模式仍无需登录，也不依赖 Control。

## 新部署

将 `nexus` 与 `nexus-control` 放在同级目录，在 `nexus/.env` 至少配置：

```env
AUTH_INIT_OWNER_USERNAME=admin
AUTH_INIT_OWNER_DISPLAY_NAME=Admin
AUTH_INIT_OWNER_PASSWORD=change-me-now
CONTROL_SERVICE_TOKEN=replace-with-openssl-rand-hex-32
HOST_DATA_DIR=/srv/nexus/data
```

服务凭据可用 `openssl rand -hex 32` 生成。随后运行：

```bash
make start
```

Compose 会先启动 Control，再启动 Nexus Server。默认 SQLite 数据库位于 `.nexus/control/data/control.db`，服务凭据和签名私钥位于 `.nexus/control/`；Nexus 只能读取 `.nexus/control-public/control-signing.pub`。两者仍属于同一个 Nexus 状态根，但不是同一个数据库或写入权威。

集群滚动升级时先迁移并升级 Control，再升级 Nexus 副本；新版 Nexus 会把 Principal 中的 entitlement 作为必填签名事实校验。整栈停机升级仍直接使用 `make start`。

多人或集群部署可让 Control 使用 PostgreSQL：

```env
CONTROL_DATABASE_DRIVER=postgres
CONTROL_DATABASE_URL=postgres://nexus_control:password@postgres:5432/nexus
```

Control 固定使用 `control` schema。数据库账号需要能创建该 schema，或由管理员预先创建并授权；`.nexus/control/` 仍保存服务凭据和签名密钥。

### 启用 Team 前将 Control SQLite 切到 PostgreSQL

Relay Team overlay 会强制 Control 使用 PostgreSQL，不能直接指向空库启动，否则现有账号权威会消失。先停止 Control、Nexus、Relay 与入口写入，备份整个 `.nexus`、原 `control.db`、Control 密钥目录和目标 PostgreSQL。完成数据库角色与 `control` schema 授权后，在 Nexus `.env` 设置 PostgreSQL 地址，再用完整的 base + Relay Compose 文件列表执行：

```bash
docker compose --env-file .env \
  -f deploy/docker-compose.yml \
  -f "$RELAY_ROOT/deploy/docker-compose.yml" \
  run --rm --no-deps \
  -v /srv/nexus/data/.nexus/control/data:/import:ro \
  control import-control-sqlite --source /import/control.db
```

目标 `control` schema 必须没有业务数据，重复导入会拒绝。迁移保留 Deployment ID、User ID、账号资料和状态、密码哈希、Membership、套餐与成员额度；旧 Session、密码修改回执和身份失效事件不迁移，因此所有用户需重新登录。随后查询 `control.deployments` 的唯一 `deployment_id`，将它写入 Relay `.env` 的 `RELAY_DEPLOYMENT_ID`，再按 Control、Relay、Nexus、Nginx 顺序启动并验收。原 Control 私钥、公钥和 service token 位于数据库外，必须继续挂载原目录。

回滚时先再次停写，恢复切换前的 Nexus `.env`、完整 state root 和 SQLite 备份；上线后新增的账号、权限与 Relay 共享消息不会自动写回旧 SQLite，产生新写入后只能走对账迁移，不能用覆盖数据库冒充无损回滚。

任何 Relay PostgreSQL 恢复同样必须先停入口写入、Nexus 与 Relay，禁止让旧 Relay 进程跨越数据库时间线。恢复后只启动新 Relay 进程，并在重新放流量前通过 bootstrap 确认 `stream_epoch` 已变化；所有旧 cursor 随后走全量 snapshot。

如果希望从 Web 初始化首个 owner，不设置 `AUTH_INIT_OWNER_PASSWORD`，改为配置至少 32 个字符的 `CONTROL_SETUP_TOKEN`，启动后访问 `/setup`。Setup code 只随这次同源请求发送给 Control，不进入 Nexus Server，也不会保存在浏览器。登录后，owner/admin 可在「设置 / 运维」管理 Deployment 成员、套餐与成员额度。

## 迁移现有 Web 账号

迁移期间不能同时运行旧 Nexus 认证写入和 Control。先安排停机窗口，并备份 Nexus 数据库与整个 `.nexus/users/`。

1. 停止旧服务并准备 Control 目录。

   ```bash
   make stop
   make prepare-host-data
   cp -p /srv/nexus/data/.nexus/app/data/nexus.db /srv/nexus/data/.nexus/app/data/nexus.db.before-control
   ```

2. 构建镜像后，以只读方式挂载旧 Nexus 数据库并导入。

   ```bash
   make build
   docker compose --env-file .env -f deploy/docker-compose.yml run --rm --no-deps \
     -v /srv/nexus/data/.nexus/app/data/nexus.db:/import/nexus.db:ro \
     control import-nexus --source /import/nexus.db
   ```

3. 启动新服务并使用原用户名、密码重新登录。

   ```bash
   make start-no-build
   ```

导入保留已有 User ID、角色、资料、Argon2id 密码哈希、订阅套餐与成员额度，但不复制旧 Session。Nexus 升级迁移先把原 `users` 资料复制为本地 `owner_profiles` 读模型；首次登录时再建立 `(deployment_id, control_user_id) -> local_owner_key`。匹配到迁移前 User ID 时沿用原 owner key，因此 Agent、workspace、transcript 和 Provider 数据不移动。旧 `users`、密码、认证 Session、`subscription_plans` 与 `user_subscriptions` 表只保留为一次性迁移输入，运行时不再读写，也不能作为回退权威。

如果账号已经通过不含订阅域的早期 Control 版本迁移，升级数据库后仍在停机窗口执行一次订阅补导：

```bash
docker compose --env-file .env -f deploy/docker-compose.yml run --rm --no-deps \
  -v /srv/nexus/data/.nexus/app/data/nexus.db:/import/nexus.db:ro \
  control import-nexus-subscriptions --source /import/nexus.db
```

该命令不改写账号和密码，只把旧套餐与成员额度写入当前 Control Deployment，并为 active 成员追加 `entitlement_changed`。Nexus 启动后会从本地持久游标重放这些事件；额度从下一次 Agent 请求开始生效，正在执行的 Agent 不会被中断。

## 验收与回滚

验收至少确认：旧 Session 已失效、原账号可登录、原套餐与成员额度可见、登录后仍能看到原 Agent 和 workspace、单次登出只关闭该浏览器 Session、头像变更刷新 Web 资料且不中断 Agent、套餐调整会同步到 Nexus 并只影响后续请求、角色变更或停用后对应 WebSocket 与 runtime 会被主动关闭、停止 Control 后已过期身份不能继续访问、Desktop 本地模式无需账号或密码且不受影响。

如需回滚，先停止新栈，再恢复切换前数据库和旧版本程序。切换后产生的业务写入不在旧快照中，不能靠直接覆盖数据库无损回滚；已有用户写入后应先导出或对账。不要让旧 Nexus 认证和 Control 同时写账号。
