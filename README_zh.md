<div align="center">

# Nexus

[![Go 1.26+](https://img.shields.io/badge/go-1.26+-00ADD8.svg)](https://go.dev/)
[![Node.js 22+](https://img.shields.io/badge/node-22+-339933.svg)](https://nodejs.org/)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-yellow.svg)](https://www.apache.org/licenses/LICENSE-2.0)

<p align="center">
  <strong>中文</strong> | <a href="./README.md">English</a>
</p>

</div>

---

<div align="center">
<img src="./docs/image/launcher.png" alt="Nexus 工作台" width="90%">
</div>

---

## 项目概述

Nexus 是面向企业、科研团队及开发者的多智能体协作平台。通过可独立命名、拥有自主工作区和持久记忆的 AI 代理（Agent），实现跨会话任务协作和知识积累。可在“房间”中组织多智能体围绕复杂任务进行讨论、分工和汇总，也可与单一智能体进行专注任务处理。

相比传统的单体 AI 办公工具，Nexus 提供：

* 多代理协作：支持多智能体同时参与任务，协同生成结果
* 持续记忆与知识积累：工作成果在 Agent 工作区内沉淀，可跨会话延续
* 主动执行能力：Agent 可通过定时任务、心跳机制和环境感知主动推进工作
* 灵活扩展能力：Skill 插件扩展和 Connector 集成外部服务（GitHub、Gmail 等）

Nexus 将智能体管理、任务协作和外部服务连接整合于一个统一平台，构建现代化的 AI 协同生态。

---

## 核心特性


| **分类**         | **特性**                                         | **优势**                           |
| ---------------- | ------------------------------------------------ | ---------------------------------- |
| **Agent 管理**   | 独立身份、工作区、技能配置，记忆跨会话保留       | 提供连续性工作流程，减少重复输入   |
| **房间协作**     | 多 Agent 协作，支持 @ 提及、定向回复、多线程推进 | 高效团队协作，分工明确             |
| **主动执行**     | 心跳、定时任务、环境感知                         | Agent 可主动推进任务，而非被动响应 |
| **技能与连接器** | Skill 扩展能力，Connector 接入外部服务           | 可扩展业务逻辑，与企业现有系统集成 |
| **部署灵活性**   | Web 界面、Docker/源码服务端、macOS/Windows 原生桌面 | 满足多平台、多场景部署需求         |


---

## 快速开始

### 选择 Agent runtime 后端

Nexus 支持两种 Agent runtime 后端：`nxs`（Nexus 原生）和 `claude`（Claude Code）。`nxs` 作为默认后端随 Nexus 提供，直连 LLM API（支持 Anthropic Messages 与 OpenAI Chat Completions 协议），并通过环境变量 `NEXUSCTL_COMMAND_PATH` 获取明确的 `nexusctl` 命令路径。

`claude` 后端通过 Claude Code 运行 Agent。使用该后端时，需单独安装 Claude Code，将 Agent runtime 切换为 `claude`，并确保 `claude` 在后端机器的 `PATH` 中可用。

```bash
# macOS / Linux / WSL
curl -fsSL https://claude.ai/install.sh | bash

# 也可以使用 npm 安装
npm install -g @anthropic-ai/claude-code
```

Windows PowerShell：

```powershell
irm https://claude.ai/install.ps1 | iex
```

也可以使用 WinGet：

```powershell
winget install Anthropic.ClaudeCode
```

### 桌面 App

* macOS：`Nexus-macos-<version>-<build>.dmg`
* Windows：`NexusSetup-<version>-<build>.exe`

安装前校验对应的 `.sha256`。桌面 App 本地数据统一存放在 `~/.nexus`。

### 服务端部署

#### Docker 部署

服务端部署推荐使用 Docker Compose：

```bash
cat > .env <<'EOF'
AUTH_INIT_OWNER_PASSWORD=your-password
HTTP_PORT=80
HOST_DATA_DIR=./data
# 可选：源码部署需手动设置，Docker 留空会自动生成并保存到数据目录。
CONNECTOR_CREDENTIALS_KEY=
# 可选：服务器出站代理，覆盖后端 IM/OAuth 的 HTTP 和 WebSocket 请求。
HTTPS_PROXY=
NO_PROXY=localhost,127.0.0.1,::1,nexus,nginx
EOF

make start
```

打开 `http://localhost`。默认 compose 只暴露 HTTP；生产 HTTPS 建议在外层网关或负载均衡上终止 TLS，再转发到该 HTTP 入口。

IM 通道机器人凭据建议在 Web App 的「能力 / IM 通道」里配置。容器启动时会从数据库重新加载这些配置；`.env` 里的 `DISCORD_BOT_TOKEN` 和 `TELEGRAM_BOT_TOKEN` 只作为历史系统级兜底入口保留。

Docker 复用宿主本地代理时，`127.0.0.1` / `localhost` 代理地址默认会在容器入口改写为 `host.docker.internal`。如果容器需要和桌面 App `.env` 不同的值，可使用 `NEXUS_DOCKER_HTTPS_PROXY`、`NEXUS_DOCKER_HTTP_PROXY`、`NEXUS_DOCKER_ALL_PROXY`、`NEXUS_DOCKER_NO_PROXY` 或 `NEXUS_DOCKER_DATABASE_URL` 单独覆盖。

启用 IM 通道时，同一个数据库建议只运行一个后端 worker。个人微信 iLink、Telegram 轮询、Discord Gateway、钉钉 Stream 和飞书长连接都维护进程内连接；同库多副本会导致重复轮询，或一个副本抢走另一个副本的更新。

如果不是 Docker 部署，需要自己生成 connector 凭据加密密钥：

```bash
openssl rand -base64 32
```

#### 源码部署：

```bash
make install
cd web && pnpm build && cd ..
AUTH_INIT_OWNER_PASSWORD=your-password PORT=8010 go run ./cmd/nexus-server
```

### 本地开发

```bash
make install
make dev
```

后端在 `http://localhost:8010` 启动，前端开发服务在 `http://localhost:3000` 启动。


---

## 核心概念

| 概念 | 说明 |
|------|------|
| **Agent** | 系统成员。有身份、工作区、技能，记忆跨会话保留 |
| **Room** | 协作容器。Agent 和人在共享上下文里一起工作 |
| **DM** | 与单个 Agent 的持续会话，运行状态完整保留 |
| **Workspace** | 每个 Agent 独立的文件目录，自主沉淀工作产出 |
| **Skill** | 安装到 Agent 的能力扩展，内置或自定义均可 |
| **Connector** | 管理 OAuth 应用配置与外部服务账号连接 |
| **主智能体** | 系统保留 Agent，负责默认入口与平台级编排 |

---

## 许可证

Apache License 2.0 · [LICENSE](./LICENSE)
