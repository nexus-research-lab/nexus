# Nexus Core

多 Agent 协作控制台。

后端负责 Agent Runtime、会话、权限、Workspace、Memory、IM Channel；前端负责 Launcher、DM、Room、Contacts 等协作界面。

## 核心能力

- 多 Agent：独立配置、独立 workspace、独立会话资产
- 实时会话：WebSocket 消息流、中断、权限审批
- Workspace：文件树浏览、读写、创建、重命名、删除
- Memory：`MEMORY.md` + `memory/*.md`
- 多入口：Web / Discord / Telegram
- 持久化：SQLite + 本地文件

## 技术栈

- 后端：Go 1.24+、chi、coder/websocket、Goose、Claude Agent SDK（Go）
- 前端：React 19、Vite 7、React Router 7、TypeScript 5、Zustand、Tailwind CSS 4

## 项目结构

```text
.
├── cmd/                 # Go 服务入口（server / migrate / ctl / tsgen）
├── internal/            # Go 后端分层实现
├── db/                  # Goose migrations（sqlite / postgres）
├── web/                 # React + Vite 前端
├── docs/                # 技术文档
├── deploy/              # Docker / nginx / compose
├── skills/              # 内置 Skill 定义
└── makefile             # 常用命令
```

## 快速开始

### 1. 环境

- Go 1.24+
- Node.js 20+
- npm

### 2. 配置

```bash
cp env.example .env
cp web/env.example web/.env.local
```

本地开发推荐：

```bash
# web/.env.local
VITE_WS_URL=/agent/v1/chat/ws
VITE_API_URL=/agent/v1
```

Go 主线的运行时 Provider 只通过 Settings 页面维护，不再通过 `.env` 配置 `auth_token / base_url / model`。

如果要把服务放到公网，建议启用浏览器登录：

```bash
AUTH_SESSION_TTL_HOURS=168
AUTH_COOKIE_SECURE=true
AUTH_COOKIE_SAMESITE=lax
```

首个 owner 账户通过 CLI 初始化：

```bash
go run ./cmd/nexusctl auth init-owner --username admin --password change-this-password
```

认证说明：

- 登录成功后后端会签发 `HttpOnly` 会话 Cookie，并在服务端保存会话记录
- 退出登录会立即撤销当前会话，不再依赖前端单纯删除 Cookie
- 反向代理生产环境优先使用前后端同源部署
- `ACCESS_TOKEN` 仍可作为机器调用的 Bearer Token 兼容入口

数据库默认使用本地 SQLite，通常不需要额外配置：

```bash
# .env
DATABASE_DRIVER=sqlite
DATABASE_URL=sqlite:////$HOME/.nexus/data/nexus.db
```

如果不在 `.env` 中显式填写 `DATABASE_URL`，默认会使用 `~/.nexus/data/nexus.db`。

### 3. 安装

#### Private Go SDK dependency

当前后端依赖私有模块 `github.com/nexus-research-lab/nexus-agent-sdk-go`。
在干净环境执行 `make install` 前，至少需要先满足下面三个前提：

1. `go.mod` 里不能保留开发机本地绝对路径的 `replace github.com/nexus-research-lab/nexus-agent-sdk-go => /...`
2. Go 需要把该组织下模块视为私有模块：
   ```bash
   go env -w GOPRIVATE=github.com/nexus-research-lab/*
   go env -w GONOSUMDB=github.com/nexus-research-lab/*
   ```
3. Git 需要具备对 `github.com/nexus-research-lab/*` 的非交互访问能力（PAT 或 SSH 任选其一）

PAT 示例：

```bash
git config --global url."https://<github-token>@github.com/".insteadOf https://github.com/
```

SSH 示例：

```bash
git config --global url."git@github.com:".insteadOf https://github.com/
ssh -T git@github.com
```

如果你之前已经在错误配置下跑过 `go mod tidy` / `go mod download`，Go 可能已经在
`$GOMODCACHE/pkg/mod/cache/vcs` 里缓存了一份指向 `https://github.com/...` 的旧远端。
这时即使后面补了 SSH 或 PAT，仍然会继续报：

```text
fatal: could not read Username for 'https://github.com': terminal prompts disabled
```

这种情况先清缓存，再重新执行 `go mod tidy`：

```bash
go clean -modcache
go mod tidy
```

如果你不想清整个 module cache，也可以只删除对应的 VCS 缓存目录后再重试。

如果上述前提未满足，`make install` 会在执行 `go mod tidy` 之前直接失败，并给出明确提示，而不是在更深层的位置报模糊的 GitHub 认证错误。

```bash
make install
```

### 4. 初始化数据库（首次启动）

默认 Go 开发链路会把 SQLite 放在 `~/.nexus/data/nexus.db`。

推荐首次启动顺序：

```bash
make db-init
make dev
```

说明：

- `make db-init` 会执行 Goose 迁移
- `make dev` / `make run-backend` 启动 Go 后端前会先执行迁移
- Docker 部署会在容器启动时通过 `deploy/entrypoint.sh` 自动执行 Goose 迁移

如果你看到数据库 schema 不匹配，优先直接重建本地 SQLite 文件：

```bash
rm -f ~/.nexus/data/nexus.db
make db-init
```

### 5. 启动

```bash
make dev
```

访问地址：

- 前端：`http://localhost:3000`
- 后端：`http://localhost:8010`

分开启动：

```bash
make run-backend
make run-web
```

## 常用命令

```bash
make help
make install
make dev
make run-backend
make run-web
make build
make prepare-host-data
make start
make logs
make stop
```

Docker 构建与启动命令默认使用当前用户执行。
如果当前用户还没有 Docker 权限，请先完成服务器上的 Docker 用户权限配置，再重新登录终端。
`Makefile` 会自动加载仓库根目录 `.env`，因此 `HOST_DATA_DIR`、`TAG`、`DATABASE_URL` 等变量既可以写进 `.env`，也可以临时通过命令行覆盖。

生产部署默认使用单一宿主机根目录变量 `${HOST_DATA_DIR:-./data}`。
其中 `${HOST_DATA_DIR}/.nexus` 挂载到容器内 `/home/agent/.nexus`，`${HOST_DATA_DIR}/.claude` 挂载到容器内 `/home/agent/.claude`，`${HOST_DATA_DIR}/.claude.json` 挂载到容器内 `/home/agent/.claude.json`。
如果要把数据统一放到宿主机目录 `/data`，启动前执行：

```bash
export HOST_DATA_DIR=/data
make prepare-host-data
```

`make prepare-host-data` 会直接在宿主机执行 `mkdir/chown/chmod`，确保 `.nexus`、`.claude` 和 `.claude.json` 都具备容器内 `agent(1001:1001)` 的读写权限。它已经是 `make start` 的前置依赖，所以常规部署直接执行 `make start` 即可。
如果当前用户没有修改属主的权限，默认会通过 `sudo` 执行；如果已经是 root 或不需要 `sudo`，可以覆盖：

```bash
HOST_SUDO= make prepare-host-data
```

## 关键配置

### 后端

- `DATABASE_DRIVER`
- `DATABASE_URL`：默认开发值为 `sqlite:////$HOME/.nexus/data/nexus.db`
- `LOG_LEVEL`
- `LOG_FORMAT`：开发环境建议 `pretty`，生产环境建议 `json`
- `LOG_STDOUT`
- `LOG_FILE_ENABLED`
- `LOG_PATH`
- `LOG_ROTATE_DAILY`
- `LOG_MAX_SIZE_MB`
- `LOG_MAX_AGE_DAYS`
- `LOG_MAX_BACKUPS`
- `LOG_COMPRESS`
- `WORKSPACE_PATH`
- `DEFAULT_AGENT_ID`
- `NEXUS_APT_ALLOWLIST`
- `AUTH_SESSION_TTL_HOURS`
- `AUTH_COOKIE_SECURE`
- `ACCESS_TOKEN`
- `DISCORD_ENABLED`
- `DISCORD_BOT_TOKEN`
- `TELEGRAM_ENABLED`
- `TELEGRAM_BOT_TOKEN`
- `CONNECTOR_OAUTH_REDIRECT_URI`：默认 `http://localhost:3000/capability/connectors/oauth/callback`，必须与各连接器开发者后台登记的回调 URI 完全一致。
- `CONNECTOR_*_CLIENT_ID` / `CONNECTOR_*_CLIENT_SECRET`：可作为部署级默认 OAuth 应用配置；终端用户也可以在连接器详情页配置自己的 OAuth 应用，用户配置会覆盖环境变量。目前 catalog 中仅 `github` 上架，其余 provider 留作占位。
- `CONNECTOR_CREDENTIALS_KEY`：32 字节 base64 AES-GCM 密钥，用于加密 connector token 与用户配置的 OAuth Client Secret。生成命令：`openssl rand -base64 32`。Dev 与 Prod 环境应使用不同的 key。

Provider 运行时的 `auth_token / base_url / model` 由 Settings 页面写入数据库中的 Provider 配置表后生效，
Agent 自身只保存 `provider` 选择，不再保存独立 `model`。

日志默认会同时输出到标准输出和 `~/.nexus/logs/logger.log`。
文件日志按天切主文件，并在单日内按大小继续滚动，默认保留 7 天 / 7 个备份并开启压缩。
生产 Docker 默认会把 `LOG_PATH` 设为 `/home/agent/.nexus/logs/logger.log`，而不是目录路径。

### 前端

- `VITE_API_URL`
- `VITE_WS_URL`

## 存储

- SQLite：结构化元数据（Agent、Session、Room、Skill 等）
- `~/.nexus/workspace/`：workspace 根目录
- `<workspace>/.agents/`：Agent 运行态、Session、消息日志、成本账本

每个 Agent 初始化时会创建：

- `AGENTS.md`
- `USER.md`
- `MEMORY.md`
- `RUNBOOK.md`
- `memory/README.md`

## 主要接口

- `GET /agent/v1/agents`
- `POST /agent/v1/agents`
- `PATCH /agent/v1/agents/{agent_id}`
- `GET /agent/v1/agents/{agent_id}/workspace/files`
- `GET /agent/v1/sessions`
- `POST /agent/v1/sessions`
- `GET /agent/v1/rooms/{room_id}/conversations/{conversation_id}/messages`
- `GET /agent/v1/skills` — 技能市场列表
- `POST /agent/v1/skills/{name}/install` — 安装技能
- `POST /agent/v1/skills/{name}/uninstall` — 卸载技能
- `WS /agent/v1/chat/ws`

## 文档

- 技术文档：`docs/nexus-technical-doc.md`
- 前端说明：`web/README.md`
- 变更记录：`CHANGELOG.md`
