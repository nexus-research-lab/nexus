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

- 后端：Python 3.11+、FastAPI、Pydantic v2、Claude Agent SDK
- 前端：React 19、Vite 7、React Router 7、TypeScript 5、Zustand、Tailwind CSS 4

## 项目结构

```text
.
├── agent/               # 后端服务、运行时、存储、工作区、通道
│   ├── api/             # API 路由层
│   │   └── capability/  # 能力市场 API（技能等）
│   ├── service/         # 业务逻辑层
│   │   └── capability/  # 能力模块
│   │       ├── skills/      # 技能市场：目录、安装、分发
│   │       ├── connectors/  # 连接器（预留）
│   │       ├── scheduled/   # 定时任务（预留）
│   │       ├── channels/    # 渠道能力（预留）
│   │       └── pairings/    # 配对能力（预留）
│   └── storage/         # 持久化（SQLite + 文件）
├── web/                 # React + Vite 前端
├── docs/                # 技术文档
├── deploy/              # Docker / nginx / compose
├── skills/              # 内置 Skill 定义
├── env.example          # 后端环境变量模板
├── main.py              # 后端入口
└── makefile             # 常用命令
```

## 快速开始

### 1. 环境

- Python 3.11+
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

后端至少需要：

```bash
AUTH_LOGIN_PASSWORD=change-this-password
```

Provider 现在只在 `Settings -> Providers` 里动态维护。
服务启动后，至少需要在该页面配置一个启用的默认 Provider，Agent 对话才会真正可用。

如果要把服务放到公网，建议同时开启浏览器登录：

```bash
# .env
AUTH_LOGIN_USERNAME=admin
AUTH_LOGIN_PASSWORD=change-this-password
AUTH_SESSION_TTL_HOURS=168
AUTH_COOKIE_SECURE=true   # HTTPS 反向代理下开启
AUTH_COOKIE_SAMESITE=lax
BACKEND_CORS_ORIGINS=https://your-domain.com
```

说明：

- 登录成功后后端会签发 `HttpOnly` 会话 Cookie，并在服务端保存会话记录
- 退出登录会立即撤销当前会话，不再依赖前端单纯删除 Cookie
- 反向代理生产环境优先使用前后端同源部署
- 如果仍然保留旧的 `ACCESS_TOKEN`，后端也会继续兼容 Bearer Token 调用

本地开发默认使用 SQLite，通常不需要额外配置：

```bash
# .env
DATABASE_URL=sqlite+aiosqlite:///~/.nexus/data/nexus.db
```

如果你直接复制 `env.example`，本地数据库文件默认就在 `cache/data/nexus.db`。
生产 Docker 部署则默认落到 `${HOST_DATA_DIR}/.nexus/data/nexus.db`。

### 3. 安装

```bash
make install
```

### 4. 初始化数据库（首次启动）

默认数据库文件为 `~/.nexus/data/nexus.db`。

推荐首次启动顺序：

```bash
make db-init
make dev
```

说明：

- `make db-init` 会执行 Alembic 迁移
- `make dev` / `make run-backend` 启动后端时也会尝试先执行迁移
- Docker 部署会在容器入口脚本中自动写入 Claude 配置并执行数据库迁移

如果你看到类似 `table agents already exists` 的报错，通常说明当前 SQLite 文件里的表已经存在，但 `alembic_version` 还没有登记版本号。可以按下面两种方式处理：

如果本地数据库可以直接重建：

```bash
rm -f ~/.nexus/data/nexus.db
make db-init
```

如果想保留现有本地数据：

```bash
.venv/bin/python -m alembic stamp head
make db-init
```

### 5. 启动

```bash
make dev
```

访问地址：

- 前端：`http://localhost:3000`
- 后端：`http://localhost:8010`
- Swagger：`http://localhost:8010/docs`

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
`Makefile` 会自动加载仓库根目录 `.env`，所以 `HOST_DATA_DIR`、`TAG` 等变量既可以手工 `export`，也可以直接写进 `.env`。

生产部署默认使用单一宿主机根目录变量 `${HOST_DATA_DIR:-./data}`。
由于 Compose 文件位于 `deploy/` 目录下，如果你不显式传入 `HOST_DATA_DIR`，默认实际路径会解析为 `deploy/data`。
其中 `${HOST_DATA_DIR}/.nexus` 挂载到容器内 `/home/agent/.nexus`，`${HOST_DATA_DIR}/.claude` 挂载到容器内 `/home/agent/.claude`。
如果要把数据统一放到宿主机目录 `/data`，启动前执行：

```bash
export HOST_DATA_DIR=/data
make prepare-host-data
```

`make prepare-host-data` 会直接在宿主机执行 `mkdir/chown/chmod`，把 `.nexus` 和 `.claude` 目录准备好，并修正为容器内 `agent` 使用的 `1001:1001`。这一步已经是 `make start` 的前置依赖，所以正常部署直接执行 `make start` 即可。
如果当前用户没有修改属主的权限，`make prepare-host-data` 会通过 `sudo` 执行 `chown/chmod`；如果你已经是 root，或者不需要 `sudo`，可以覆盖：

```bash
HOST_SUDO= make prepare-host-data
```

如果需要覆盖容器运行用户映射，可以同时传入：

```bash
AGENT_UID=1001 AGENT_GID=1001 make prepare-host-data
```

生产 Compose 现在包含：

- `nexus`：Gunicorn + FastAPI，容器启动时执行 `deploy/entrypoint.sh`
- `nginx`：承载前端静态资源并反代 `/agent/` 与 WebSocket
- `nexus` 健康检查：`GET /agent/v1/health`
- `nginx` 健康检查：`GET /nginx-health`

生产容器内主进程仍然以 `agent` 用户运行，不会直接给 Agent 任意 root 权限。
如果确实需要在运行期安装 Debian 系统包，只允许通过受控入口：

```bash
sudo /usr/local/bin/nexus-apt-install --list-allowed
sudo /usr/local/bin/nexus-apt-install ripgrep
```

这条 sudo 规则是免密码的，但只放行 `nexus-apt-install` 这一条命令，不允许任意 `sudo bash`、`sudo apt` 或其它 root shell。
允许安装的包默认来自 `NEXUS_APT_ALLOWLIST`，并会写日志到 `${HOST_DATA_DIR}/.nexus/logs/system-package-install.log`。

Docker 构建默认会把 BuildKit 缓存持久化到仓库根目录 `.buildx-cache/`，用于复用 `apt`、`pip`、`npm` 下载内容和镜像构建层。
首次构建仍然会完整下载依赖，后续只要 `requirements.txt`、`package-lock.json` 和对应构建层没有失效，就不会重复拉取整套 Python / Node 依赖。

如果你希望在容器内预写 Claude Code 的全局配置，可以在 `.env` 里额外提供这些可选变量，入口脚本会同步写入 `/home/agent/.claude/settings.json`：

```bash
ANTHROPIC_AUTH_TOKEN=
ANTHROPIC_BASE_URL=
ANTHROPIC_MODEL=
ANTHROPIC_DEFAULT_SONNET_MODEL=
ANTHROPIC_DEFAULT_OPUS_MODEL=
ANTHROPIC_DEFAULT_HAIKU_MODEL=
CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=
ENABLE_TOOL_SEARCH=
CLAUDE_DANGEROUSLY_SKIP_PERMISSIONS=true
NEXUS_APT_ALLOWLIST=ca-certificates curl ffmpeg git imagemagick iputils-ping jq less procps ripgrep rsync unzip vim wget zip
```

但这只是全局回退配置。当前代码的主路径仍然是登录后在 `Settings -> Providers` 里维护 Provider，并由后端在运行时把 Provider 的 `auth_token/base_url/model` 注入 Claude SDK。

## 关键配置

### 后端

- `DATABASE_URL`：默认值为 `sqlite+aiosqlite:///~/.nexus/data/nexus.db`
- `WORKSPACE_PATH`
- `DEFAULT_AGENT_ID`
- `WEBSOCKET_ENABLED`
- `AUTH_LOGIN_USERNAME`
- `AUTH_LOGIN_PASSWORD`
- `AUTH_SESSION_TTL_HOURS`
- `AUTH_COOKIE_SECURE`
- `BACKEND_CORS_ORIGINS`
- `DISCORD_ENABLED`
- `TELEGRAM_ENABLED`

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
- `GET /agent/v1/sessions/{session_key}/messages`
- `GET /agent/v1/sessions/{session_key}/cost/summary`
- `GET /agent/v1/skills` — 技能市场列表
- `POST /agent/v1/skills/{name}/install` — 安装技能
- `POST /agent/v1/skills/{name}/uninstall` — 卸载技能
- `WS /agent/v1/chat/ws`

## 文档

- 技术文档：`docs/nexus-technical-doc.md`
- 前端说明：`web/README.md`
- 变更记录：`CHANGELOG.md`
