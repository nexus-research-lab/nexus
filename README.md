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

In Nexus, AI agents work like colleagues.

They have names, their own workspaces, and remember where you left off. You can create a room, pull in a few agents, and watch them discuss, divide tasks, and organize results around a problem. Or you can work with just one agent in a focused conversation.

---

## Features

- Agents have independent identities, workspaces, and skill configurations. Memory persists across sessions; work output accumulates over time
- Rooms let multiple agents collaborate with you through @mentions, private actions, targeted replies, and multi-threaded workflows
- Through heartbeat, scheduled tasks, and environment awareness, agents can proactively drive work forward instead of just responding
- Skills extend capabilities; Connectors integrate external services (GitHub, Gmail, LinkedIn, X, Instagram, Shopify)
- Supports web interface, Linux/Windows server deployment, and native macOS desktop app

---

## Quick Start

### Install Claude Code

Nexus currently runs agents through `nexus-agent-sdk-bridge`, which launches Claude Code on the machine running the backend. Install Claude Code first and make sure `claude` is available in `PATH`.

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

After installation, run `claude` once and complete login. On native Windows, Git for Windows is recommended so Claude Code can use its Bash tool; without it, Claude Code falls back to PowerShell. See the [official Claude Code setup guide](https://code.claude.com/docs/en/getting-started) for the latest platform-specific instructions.

### Run a Release Package

```bash
# Linux x86_64 example
tar -xzf nexus-v0.1.8-linux-amd64.tar.gz
cd nexus-v0.1.8-linux-amd64

# Initialize database and create admin account
./bin/nexus-migrate up
printf '%s\n' 'your-password' | ./bin/nexusctl auth init-owner --username admin --password-stdin

# Start
./run-nexus
```

Open `http://localhost:8010` and sign in.

Windows packages include `run-nexus.cmd`:

```bat
bin\nexus-migrate.exe up
echo your-password| bin\nexusctl.exe auth init-owner --username admin --password-stdin
run-nexus.cmd
```

macOS app packages are published in the same GitHub Release as `Nexus-macos-<version>-<build>.dmg`. The current app package is ad-hoc signed and not notarized; verify the sha256 file before installing, then use Finder right-click Open for trusted builds if macOS blocks the first launch.

To upgrade a release package, open Settings in the Web UI and use the release download link. Download the matching package for your platform, stop Nexus, replace the extracted package directory, then run migration and start Nexus again.

### Docker

```bash
docker build -f deploy/Dockerfile -t nexus:latest .
docker run -d \
  -p 8010:8010 \
  -v nexus-data:/home/agent/.nexus \
  --name nexus \
  nexus:latest
```

Create the admin account:

```bash
printf '%s\n' 'your-password' | docker exec -i nexus nexusctl auth init-owner --username admin --password-stdin
```

### Local Development

```bash
make install
make dev
```

The backend starts at `http://localhost:8010`, the frontend dev server at `http://localhost:3000`. Both run independently with hot reload.

Requirements: Go 1.26+, Node.js 22+, pnpm 9.15+, and Claude Code available as `claude`

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

## Built-in Skills

| Skill | Description |
|-------|-------------|
| `imagegen` | Generate images via an image-generation provider and save results to the workspace |
| `nexus-manager` | Operate Nexus agents, rooms, sessions, and workspaces from an agent context |
| `room-playbook` | Provide fixed rules and operation guides for room collaboration |
| `scheduled-task-manager` | Manage scheduled tasks and heartbeat-style follow-up tasks |
| `memory-manager` | Maintain project memory files through a structured workflow |

---

## License

Apache License 2.0 · [LICENSE](./LICENSE)
