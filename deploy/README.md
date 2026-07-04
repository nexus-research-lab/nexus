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

SSL_DOMAINS=www.example.com
SSL_EMAIL=
```

`NGINX_SSL_CERTIFICATE` 和 `NGINX_SSL_CERTIFICATE_KEY` 是 nginx 容器内路径。宿主机证书实际存放在 `${HOST_DATA_DIR}/certs`，ACME HTTP-01 challenge 文件存放在 `${HOST_DATA_DIR}/acme`。

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
