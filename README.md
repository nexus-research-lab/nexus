<div align="center">

# Nexus

[![Go 1.26+](https://img.shields.io/badge/go-1.26+-00ADD8.svg)](https://go.dev/)
[![Node.js 22+](https://img.shields.io/badge/node-22+-339933.svg)](https://nodejs.org/)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-yellow.svg)](https://www.apache.org/licenses/LICENSE-2.0)

<p align="center">
  <a href="./README_zh.md">中文</a> | <strong>English</strong>
</p>

</div>

---

<div align="center">
<img src="./docs/image/launcher.png" alt="Nexus workspace" width="90%">
</div>

---

## Overview

Nexus is a multi-agent collaboration platform for enterprises, research teams, and developers. Agents can be named independently, own their own workspaces, and keep persistent memory, so task context and knowledge can continue across sessions. Rooms can organize multiple agents to discuss, divide work, and synthesize results around complex tasks, while DMs support focused work with a single agent.

Compared with traditional single-agent AI office tools, Nexus provides:

- Multi-agent collaboration: multiple agents can participate in the same task and produce results together
- Persistent memory and knowledge accumulation: work output is retained in each Agent workspace and can continue across sessions
- Proactive execution: agents can drive work forward through scheduled tasks, heartbeat tasks, and environment awareness
- Flexible extensibility: Skills extend agent capabilities, and Connectors integrate external services such as GitHub and Gmail

Nexus brings agent management, task collaboration, and external service connections into one unified platform for a modern AI collaboration ecosystem.

---

## Features

| **Category** | **Capabilities** | **Benefit** |
|--------------|------------------|-------------|
| **Agent Management** | Independent identity, workspace, skill configuration, and cross-session memory | Continuous workflows with less repeated context |
| **Room Collaboration** | Multi-agent collaboration with @mentions, targeted replies, and multi-threaded progress | Clear division of work for team-style collaboration |
| **Proactive Execution** | Heartbeats, scheduled tasks, and environment awareness | Agents can move work forward instead of only responding |
| **Skills & Connectors** | Skill extensions and Connector integrations with external services | Extensible business logic and integration with existing systems |
| **Deployment Flexibility** | Web UI, Docker/source server deployment, and native macOS/Windows desktop apps | Fits multiple platforms and deployment scenarios |

---

## Quick Start

### Choose an Agent runtime backend

Nexus supports two Agent runtime backends: `nxs` (native Nexus) and `claude` (Claude Code). `nxs` is bundled as the default backend; it talks to LLM APIs directly (both Anthropic Messages and OpenAI Chat Completions protocols are supported) and receives an explicit `nexusctl` command path through the `NEXUSCTL_COMMAND_PATH` environment variable.

The `claude` backend runs agents through Claude Code. To use it, install Claude Code separately, switch the agent runtime to `claude`, and make sure `claude` is available in the backend machine's `PATH`.

```bash
# macOS / Linux / WSL
curl -fsSL https://claude.ai/install.sh | bash

# Alternative npm install
npm install -g @anthropic-ai/claude-code
```

On Windows PowerShell:

```powershell
irm https://claude.ai/install.ps1 | iex
```

Or install with WinGet:

```powershell
winget install Anthropic.ClaudeCode
```

### Desktop Apps

- macOS: `Nexus-macos-<version>-<build>.dmg`
- Windows: `NexusSetup-<version>-<build>.exe`

Verify the matching `.sha256` file before installing. Desktop app data is stored under `~/.nexus`.

### Server Deployment

#### Docker Deployment

Docker Compose is recommended for server deployment:

```bash
cat > .env <<'EOF'
AUTH_INIT_OWNER_PASSWORD=your-password
HTTP_PORT=80
HOST_DATA_DIR=./data
# Optional: source deployments must set this manually; Docker generates and persists one when empty.
CONNECTOR_CREDENTIALS_KEY=
# Optional: server-side outbound proxy for backend IM/OAuth HTTP and WebSocket requests.
HTTPS_PROXY=
NO_PROXY=localhost,127.0.0.1,::1,nexus,nginx
EOF

make start
```

Open `http://localhost`. The default compose stack only exposes HTTP; terminate production TLS at an outer gateway or load balancer and forward to this HTTP entrypoint.

Configure IM channel credentials in the web app under Capability / Channels. The container reloads saved channel configs from the database on startup; `DISCORD_BOT_TOKEN` and `TELEGRAM_BOT_TOKEN` in `.env` are only legacy system-level fallbacks.

When reusing a local host proxy for Docker, `127.0.0.1` / `localhost` proxy hosts are rewritten to `host.docker.internal` by default. Use `NEXUS_DOCKER_HTTPS_PROXY`, `NEXUS_DOCKER_HTTP_PROXY`, or `NEXUS_DOCKER_DATABASE_URL` when the container needs values that differ from the desktop app `.env`.

For non-Docker deployments, generate the connector credentials encryption key yourself:

```bash
openssl rand -base64 32
```

#### Source Deployment

```bash
make install
cd web && pnpm build && cd ..
AUTH_INIT_OWNER_PASSWORD=your-password PORT=8010 go run ./cmd/nexus-server
```

### Local Development

```bash
make install
make dev
```

The backend starts at `http://localhost:8010`, the frontend dev server at `http://localhost:3000`.

---

## Core Concepts

| Concept | Description |
|---------|-------------|
| **Agent** | A workspace member with identity, workspace, skills, and cross-session memory |
| **Room** | A collaboration space where agents and humans work in a shared context |
| **DM** | A persistent conversation with a single agent, preserving full runtime state |
| **Workspace** | An isolated file directory where each agent stores its work output |
| **Skill** | A capability extension installed on an agent — built-in or custom |
| **Connector** | Manages OAuth app configurations and external service account connections |
| **Main Agent** | A reserved system agent responsible for default entry and platform-level orchestration |

---

## License

Apache License 2.0 · [LICENSE](./LICENSE)
