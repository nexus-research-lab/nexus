<div align="center">

# Nexus

<p align="center">
  <em>Local-first multi-agent workspace with rooms, skills, automation, connectors, and private workspaces</em>
</p>

[![Go 1.26+](https://img.shields.io/badge/go-1.26+-00ADD8.svg)](https://go.dev/)
[![Node.js 22+](https://img.shields.io/badge/node-22+-339933.svg)](https://nodejs.org/)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-yellow.svg)](https://www.apache.org/licenses/LICENSE-2.0)

<p align="center">
  <a href="./README_zh.md">中文</a> | <strong>English</strong>
</p>

</div>

<div align="center">
<img src="./docs/image/launcher.png" alt="Nexus launcher" width="90%">
</div>

---

## Recent Updates (v0.1.3)

- Runnable Linux and Windows release packages now include the Go service, web UI, migrations, and built-in skills
- Image generation is available through image-generation providers, the `imagegen` skill, and inline image previews
- Room collaboration adds private actions, targeted replies, audience delivery, delayed wakeups, and room-scoped skills
- The macOS desktop dogfood path has a sidecar runtime, separate windows, desktop session credentials, and startup diagnostics

---

## Why Nexus?

**Nexus is a self-hosted workspace for coordinating humans and AI agents around real project work.**

- **Multi-agent rooms** — Mention agents, request replies, and coordinate public or private work inside a shared room
- **Direct conversations** — Work with one agent while preserving runtime state, message queues, and workspace history
- **Workspace isolation** — Keep users, agents, sessions, files, skills, and connector data scoped to the right owner
- **Runtime control** — Queue new input, interrupt running work, change permission modes, and inspect execution state
- **Skills and automation** — Install built-in skills, schedule recurring work, and wake agents for follow-up tasks
- **Connector foundation** — Manage OAuth connector credentials and prepare channel integrations from the same control plane
- **Single-origin app** — The Go service can serve the built frontend directly, so release packages start from one local port

---

## Quick Start

### Run a Release Package

```bash
# Linux x86_64 example
tar -xzf nexus-v0.1.3-linux-amd64.tar.gz
cd nexus-v0.1.3-linux-amd64

# Create the first owner account
./bin/nexus-migrate up
printf '%s\n' 'your-password' | ./bin/nexusctl auth init-owner --username admin --password-stdin

# Start Nexus
./run-nexus
```

Open `http://localhost:8010` and sign in with the owner account.

Windows packages include `run-nexus.cmd`:

```bat
bin\nexus-migrate.exe up
echo your-password| bin\nexusctl.exe auth init-owner --username admin --password-stdin
run-nexus.cmd
```

### Run From Source

```bash
make install
make dev
```

The backend listens on `http://localhost:8010`, and the Vite dev server listens
on `http://localhost:3000`.

---

## Skills Support

Nexus ships with built-in skills under `skills/`, including:

- `imagegen` — generate images and save results into the current agent workspace
- `nexus-manager` — operate Nexus agents, rooms, sessions, and workspaces from an agent context
- `room-playbook` — guide agents in room collaboration workflows
- `scheduled-task-manager` — manage scheduled and heartbeat-style tasks
- `memory-manager` — maintain project memory files through a structured workflow

---

## Features

| Feature | Description |
|---------|-------------|
| Agent Workspace | Create agents, configure providers, manage skills, and isolate workspace files |
| Direct Conversation | Chat with a single agent with streaming output, queueing, interrupts, and history |
| Room Collaboration | Coordinate multiple agents with public messages, private actions, targeted replies, and delayed wakeups |
| Runtime Permissions | Switch permission modes and keep AskUserQuestion-style user interaction available |
| Image Generation | Use image-generation providers through `nexusctl imagegen` and the `imagegen` skill |
| Scheduled Tasks | Create recurring tasks, run them manually, and inspect run history |
| Connectors | Manage OAuth connector apps and account connections |
| Channels | Preview external channel pairing and inbound message foundations |
| Workspace Files | Browse, upload, edit, download, rename, and delete agent workspace files |
| Local Deployment | Run as a Go service, Docker deployment, or a packaged Go + web bundle |

---

## Release Packages

Official release assets include source archives and runnable service packages:

| Target | Format |
|--------|--------|
| `linux-amd64` | `.tar.gz` |
| `linux-arm64` | `.tar.gz` |
| `windows-amd64` | `.zip` |

Each runnable package includes:

- `bin/nexus-server`
- `bin/nexus-migrate`
- `bin/nexusctl`
- `db/migrations`
- built-in `skills`
- built `web/dist`
- `run-nexus` or `run-nexus.cmd`

The macOS desktop app is not part of the official release packages yet.

---

## Build

Requirements:

- Go 1.26.2+
- Node.js 22+
- pnpm 9.15.2+

Common commands:

| Command | Description |
|---------|-------------|
| `make dev` | Run Go backend and frontend dev server |
| `make check` | Run Go tests, frontend lint, and frontend typecheck |
| `make db-init` | Run database migrations |
| `make gen-protocol-types` | Regenerate frontend protocol types from Go definitions |
| `make package-release` | Build a runnable Go + web package for the current platform |

Build a specific release target:

```bash
NEXUS_RELEASE_TARGET=linux-amd64 ./scripts/package-release.sh 0.1.3
NEXUS_RELEASE_TARGET=linux-arm64 ./scripts/package-release.sh 0.1.3
NEXUS_RELEASE_TARGET=windows-amd64 ./scripts/package-release.sh 0.1.3
```

Nexus uses SQLite through CGO. Cross-compilation requires the matching C
compiler; the GitHub Release workflow installs Linux ARM64 and Windows AMD64
toolchains before packaging.

---

## Go Bridge SDK Dependency

Nexus depends on the public Go bridge module:

```text
github.com/nexus-research-lab/nexus-agent-sdk-bridge
```

The bridge module contains shared client, protocol, permission, hook, and MCP
contracts. The private runtime SDK is not required for the default open-source
build.

Check access:

```bash
make check-bridge-sdk-access
```

Use a local bridge checkout during bridge development:

```bash
go mod edit -replace github.com/nexus-research-lab/nexus-agent-sdk-bridge=/path/to/nexus-agent-sdk-bridge
```

Before committing on `main`, remove the local replace:

```bash
go mod edit -dropreplace github.com/nexus-research-lab/nexus-agent-sdk-bridge
go mod tidy
```
