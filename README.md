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
ANTHROPIC_AUTH_TOKEN=your_token
ANTHROPIC_MODEL=your_model
```

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

数据库默认使用本地 SQLite，通常不需要额外配置：

```bash
# .env
DATABASE_URL=sqlite+aiosqlite:///./cache/data/data.db
```

如果不在 `.env` 中显式填写 `DATABASE_URL`，后端也会使用上面的默认值。

### 3. 安装

```bash
make install
```

### 4. 初始化数据库（首次启动）

默认数据库文件为 `cache/data/data.db`。

推荐首次启动顺序：

```bash
make db-init
make dev
```

说明：

- `make db-init` 会执行 Alembic 迁移
- `make dev` / `make run-backend` 启动后端时也会尝试先执行迁移
- Docker 部署会在容器启动时自动执行数据库初始化脚本

如果你看到类似 `table agents already exists` 的报错，通常说明当前 SQLite 文件里的表已经存在，但 `alembic_version` 还没有登记版本号。可以按下面两种方式处理：

如果本地数据库可以直接重建：

```bash
rm -f cache/data/data.db
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
make start
make logs
make stop
```

Docker 构建与启动命令默认使用当前用户执行。
如果当前用户还没有 Docker 权限，请先完成服务器上的 Docker 用户权限配置，再重新登录终端。

生产部署默认使用单一宿主机根目录变量 `${HOST_DATA_DIR:-./data}`。
其中 `${HOST_DATA_DIR}/.nexus` 挂载到容器内 `/home/agent/.nexus`，`${HOST_DATA_DIR}/.claude` 挂载到容器内 `/home/agent/.claude`。
如果要把数据统一放到宿主机目录 `/data`，启动前执行：

```bash
export HOST_DATA_DIR=/data
mkdir -p "$HOST_DATA_DIR/.nexus" "$HOST_DATA_DIR/.claude"
chown -R 1001:1001 "$HOST_DATA_DIR/.nexus" "$HOST_DATA_DIR/.claude"
```

## 关键配置

### 后端

- `ANTHROPIC_AUTH_TOKEN`
- `ANTHROPIC_BASE_URL`
- `ANTHROPIC_MODEL`
- `DATABASE_URL`：默认值为 `sqlite+aiosqlite:///./cache/data/data.db`
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
- `VITE_DEFAULT_MODEL`

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
