# Nexus Docker 部署和 SSL

这份配置默认不带任何私有证书。没有证书时 nginx 只监听 HTTP；证书文件存在后，入口脚本会生成 HTTPS 配置，并按 `NGINX_REDIRECT_HTTPS` 决定是否把 HTTP 跳转到 HTTPS。

## 环境变量

生产环境建议在仓库根目录 `.env` 中配置：

```env
HOST_DATA_DIR=/srv/nexus/data
NGINX_SERVER_NAME=www.example.com
NGINX_SSL_CERTIFICATE=/etc/nginx/certs/live/www.example.com/fullchain.pem
NGINX_SSL_CERTIFICATE_KEY=/etc/nginx/certs/live/www.example.com/privkey.pem
NGINX_REDIRECT_HTTPS=true
HTTPS_PORT=443
NEXUS_NXS_RUNTIME_RELEASE=nxs-stable
CONTROL_SERVICE_TOKEN=replace-with-openssl-rand-hex-32

SSL_DOMAINS=www.example.com
SSL_EMAIL=
```

`nexus-control` 仓库默认位于本仓同级目录；其他位置可通过 `NEXUS_CONTROL_BUILD_CONTEXT` 指定。Nginx 将同一 Web Origin 下的 `/auth/v1/*` 转发到 Control，将 `/nexus/v1/*` 转发到 Nexus Server。Control 私有目录使用独立容器 UID 1002，Nexus 进程只读取公开签名公钥。已有 Web 用户切换前按 [Control 迁移文档](../docs/operations/control-migration.md) 停止旧认证写入并导入账号。

`make deploy` 会以 fast-forward 方式同时更新当前 Nexus 仓和 `NEXUS_CONTROL_ROOT` 指向的 Control 仓，再统一构建、停止和启动三项服务，避免只更新一侧源码。自定义 Control 路径时，应同时设置 `NEXUS_CONTROL_ROOT` 与 `NEXUS_CONTROL_BUILD_CONTEXT`。

Control 默认使用 `${HOST_DATA_DIR}/.nexus/app/control/control.db`。改用 PostgreSQL 时，在 `.env` 设置 `CONTROL_DATABASE_DRIVER=postgres` 与完整的 `CONTROL_DATABASE_URL`；账号表固定位于 `control` schema。Control 本地目录仍需保留，用于服务凭据与签名密钥。

`NGINX_SSL_CERTIFICATE` 和 `NGINX_SSL_CERTIFICATE_KEY` 是 nginx 容器内路径。宿主机证书实际存放在 `${HOST_DATA_DIR}/certs`，ACME HTTP-01 challenge 文件存放在 `${HOST_DATA_DIR}/acme`。

Compose 不会因为 `env_file` 自动读取仓库根目录的 `.env` 来展开宿主机挂载路径。所有构建、启动和运维命令都显式传入根目录环境文件：

```bash
docker compose --env-file .env -f deploy/docker-compose.yml build
docker compose --env-file .env -f deploy/docker-compose.yml up -d
```

`HOST_DATA_DIR` 为必填项；未传入时 Compose 会直接失败，避免把数据误挂载到 `deploy/data`。

`make start`、`make start-no-build` 的宿主准备步骤只会初始化尚不存在的
`.nexus` 和 `.claude.json`。已有 `.nexus` 内的 UID/GID 与 POSIX ACL 由 runtime
launcher 管理，部署脚本不会递归 `chown` 或 `chmod`；运维时也不要对该状态树执行
这类递归权限重置。

`make build` / `make start` 会在构建前把 `nxs-stable` 解析成当前具体的 `nxs-v*` runtime release，再传给 Docker build。stable 指向新版 runtime 后，Docker build arg 会自动变化，旧 runtime 层缓存会失效；需要灰度或回滚时也可以直接把 `NEXUS_NXS_RUNTIME_RELEASE` 固定到具体 `nxs-v*`。

## 首次申请证书

先确认域名 A 记录已经指向服务器，且 80 端口能访问当前 nginx：

```bash
make deploy
deploy/ssl-certbot.sh check
deploy/ssl-certbot.sh issue
```

申请成功后脚本会让 nginx 重新生成配置并 reload。也可以手动重启 nginx：

```bash
docker compose --env-file .env -f deploy/docker-compose.yml restart nginx
```

验证 HTTPS：

```bash
curl -I https://www.example.com/nginx-health
```

## 自动续期

Let's Encrypt 证书有效期通常是 90 天。使用当前脚本时，续期不需要停 nginx，因为 `/.well-known/acme-challenge/` 会一直走 HTTP webroot。

建议在服务器用户 crontab 中每天跑一次：

```cron
17 3 * * * cd /srv/nexus/app && deploy/ssl-certbot.sh renew >> /srv/nexus/data/certs/renew.log 2>&1
```

先做一次 dry-run：

```bash
deploy/ssl-certbot.sh dry-run
```

如果证书最初是用 standalone 模式申请的，先在新 nginx 配置部署后强制重签一次，把 certbot renewal 配置切到 webroot：

```bash
SSL_FORCE_RENEWAL=true deploy/ssl-certbot.sh issue
```

## 多域名

先让每个域名都解析到服务器，再配置：

```env
NGINX_SERVER_NAME=example.com www.example.com
SSL_DOMAINS=example.com www.example.com
NGINX_SSL_CERTIFICATE=/etc/nginx/certs/live/example.com/fullchain.pem
NGINX_SSL_CERTIFICATE_KEY=/etc/nginx/certs/live/example.com/privkey.pem
```

HTTP-01 不支持通配符域名。需要 `*.example.com` 时改用 DNS-01。
