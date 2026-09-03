# Nexus Control 部署与迁移

服务端 Web 账号由独立的 `nexus-control` 管理。Desktop 本地模式仍无需登录，也不依赖 Control。

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

Compose 会先启动 Control，再启动 Nexus Server。默认 SQLite 数据库和签名私钥位于 `.nexus/app/control/`；Nexus 只能读取 `.nexus/app/control-public/control-signing.pub`。两者仍属于同一个 Nexus 状态根，但不是同一个数据库或写入权威。

多人或集群部署可让 Control 使用 PostgreSQL：

```env
CONTROL_DATABASE_DRIVER=postgres
CONTROL_DATABASE_URL=postgres://nexus_control:password@postgres:5432/nexus
```

Control 固定使用 `control` schema。数据库账号需要能创建该 schema，或由管理员预先创建并授权；`.nexus/app/control/` 仍保存服务凭据和签名密钥。

如果希望从 Web 初始化首个 owner，不设置 `AUTH_INIT_OWNER_PASSWORD`，改为配置至少 32 个字符的 `CONTROL_SETUP_TOKEN`，启动后访问 `/setup`。Setup code 只随这次同源请求发送给 Control，不进入 Nexus Server，也不会保存在浏览器。登录后，owner/admin 可在「设置 / 运维 / 成员」管理 Deployment 账号。

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

导入保留已有 User ID、角色、资料和 Argon2id 密码哈希，但不复制旧 Session。Nexus 升级迁移先把原 `users` 资料复制为本地 `owner_profiles` 读模型；首次登录时再建立 `(deployment_id, control_user_id) -> local_owner_key`。匹配到迁移前 User ID 时沿用原 owner key，因此 Agent、workspace、transcript 和 Provider 数据不移动。旧 `users`、密码和认证 Session 表只保留为一次性迁移输入，运行时不再读写，也不能作为回退账号权威。

## 验收与回滚

验收至少确认：旧 Session 已失效、原账号可登录、登录后仍能看到原 Agent 和 workspace、单次登出只关闭该浏览器 Session、头像变更刷新 Web 资料且不中断 Agent、角色变更或停用后对应 WebSocket 与 runtime 会被主动关闭、停止 Control 后已过期身份不能继续访问、Desktop 本地模式无需账号或密码且不受影响。

如需回滚，先停止新栈，再恢复切换前数据库和旧版本程序。切换后产生的业务写入不在旧快照中，不能靠直接覆盖数据库无损回滚；已有用户写入后应先导出或对账。不要让旧 Nexus 认证和 Control 同时写账号。
