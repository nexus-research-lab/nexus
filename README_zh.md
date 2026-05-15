<div align="center">

# Nexus

<p align="center">
  <em>本地优先的多智能体协作工作台，支持房间、技能、自动化、连接器和独立工作区</em>
</p>

[![Go 1.26+](https://img.shields.io/badge/go-1.26+-00ADD8.svg)](https://go.dev/)
[![Node.js 22+](https://img.shields.io/badge/node-22+-339933.svg)](https://nodejs.org/)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-yellow.svg)](https://www.apache.org/licenses/LICENSE-2.0)

<p align="center">
  <strong>中文</strong> | <a href="./README.md">English</a>
</p>

</div>

<div align="center">
<img src="./docs/image/launcher.png" alt="Nexus 启动台" width="90%">
</div>

---

## 最近重要更新（v0.1.3）

- Linux / Windows 运行包已包含 Go 服务、前端资源、数据库迁移和内置技能，解压后即可启动
- 图片生成链路成型，支持图片生成 Provider、`imagegen` 技能和会话内图片预览
- Room 协作动作增强，支持私域消息、定向请求回复、小范围受众投递、延迟唤醒和房间级技能
- macOS 桌面端 dogfood 链路完成第一阶段，已具备 sidecar、本地窗口、桌面会话凭据和启动诊断

---

## 为什么选择 Nexus？

**Nexus 用来把人和多个智能体组织到同一个真实工作流里。**

- **多智能体房间** — 在一个房间里 @ 智能体、请求回复、投递私域动作，让多个智能体围绕同一件事协作
- **一对一会话** — 和单个智能体持续对话，保留运行状态、待发送队列、历史记录和工作区上下文
- **工作区隔离** — 用户、智能体、会话、文件、技能和连接器数据按归属边界隔离
- **运行态可控** — 支持排队、中断、权限模式切换和执行状态查看，不把后台运行变成黑盒
- **技能与自动化** — 内置技能、定时任务和 heartbeat 可以让智能体按计划继续工作
- **连接器基础能力** — 统一管理 OAuth 应用配置和账号连接，为外部频道接入预留运行底座
- **单端口运行** — Go 服务可直接托管前端，正式运行包不需要额外启动前端服务

---

## 30 秒快速开始

### 使用发布包

```bash
# 以 Linux x86_64 包为例
tar -xzf nexus-v0.1.3-linux-amd64.tar.gz
cd nexus-v0.1.3-linux-amd64

# 初始化首个 owner 账号
./bin/nexus-migrate up
printf '%s\n' 'your-password' | ./bin/nexusctl auth init-owner --username admin --password-stdin

# 启动 Nexus
./run-nexus
```

打开 `http://localhost:8010`，使用刚创建的 owner 账号登录。

Windows 包提供 `run-nexus.cmd`：

```bat
bin\nexus-migrate.exe up
echo your-password| bin\nexusctl.exe auth init-owner --username admin --password-stdin
run-nexus.cmd
```

### 从源码启动

```bash
make install
make dev
```

后端默认监听 `http://localhost:8010`，前端开发服务默认监听 `http://localhost:3000`。

---

## Skills 支持

Nexus 内置多组技能，位于 `skills/` 目录：

- `imagegen`：生成图片并把结果保存到当前智能体工作区
- `nexus-manager`：让智能体通过命令操作 Nexus 的智能体、房间、会话和工作区
- `room-playbook`：为房间协作提供固定规则和操作指引
- `scheduled-task-manager`：管理定时任务与 heartbeat 类持续跟进任务
- `memory-manager`：按约定维护项目记忆文件

---

## 功能特性

| 功能 | 描述 |
|------|------|
| 智能体工作区 | 创建智能体、配置模型供应商、管理技能，并隔离工作目录 |
| 一对一会话 | 支持流式输出、消息排队、中断、历史记录和运行态恢复 |
| Room 协作 | 多智能体公开讨论、私域动作、定向回复、延迟唤醒和房间级规则 |
| 权限控制 | 支持多种权限模式，并保留 AskUserQuestion 类用户交互通道 |
| 图片生成 | 通过图片生成 Provider、`nexusctl imagegen` 和 `imagegen` 技能生成图片 |
| 定时任务 | 创建、编辑、手动运行定时任务，并查看运行记录 |
| 连接器 | 管理 OAuth 应用配置和账号连接 |
| 外部频道 | 提供频道配置、配对和入站消息的基础能力 |
| 工作区文件 | 浏览、上传、编辑、下载、重命名和删除智能体工作区文件 |
| 本地部署 | 支持 Go 服务、Docker 部署和 Go + Web 一体运行包 |

---

## 发布包

正式发布资产包含源码包和以下可运行服务端包：

| 平台 | 格式 |
|------|------|
| `linux-amd64` | `.tar.gz` |
| `linux-arm64` | `.tar.gz` |
| `windows-amd64` | `.zip` |

运行包内包含：

- `bin/nexus-server`
- `bin/nexus-migrate`
- `bin/nexusctl`
- `db/migrations`
- 内置 `skills`
- 构建后的 `web/dist`
- `run-nexus` 或 `run-nexus.cmd`

macOS 桌面应用暂不放进正式发布包，仍按 dogfood 链路单独验证。

---

## 构建与校验

需要准备：

- Go 1.26.2 或更高版本
- Node.js 22 或更高版本
- pnpm 9.15.2 或更高版本

常用命令：

| 命令 | 说明 |
|------|------|
| `make dev` | 同时启动 Go 后端和前端开发服务 |
| `make check` | 执行 Go 测试、前端 lint 和前端类型检查 |
| `make db-init` | 执行数据库迁移 |
| `make gen-protocol-types` | 根据 Go 协议模型重新生成前端类型 |
| `make package-release` | 为当前平台构建 Go + Web 一体运行包 |

指定目标平台构建：

```bash
NEXUS_RELEASE_TARGET=linux-amd64 ./scripts/package-release.sh 0.1.3
NEXUS_RELEASE_TARGET=linux-arm64 ./scripts/package-release.sh 0.1.3
NEXUS_RELEASE_TARGET=windows-amd64 ./scripts/package-release.sh 0.1.3
```

Nexus 通过 CGO 使用 SQLite。跨平台构建时，需要安装目标平台对应的 C 编译器；GitHub Release workflow 会在打包 Linux ARM64 和 Windows AMD64 产物前自动安装所需工具链。

---

## Go Bridge SDK 依赖

本仓库依赖公开的 Go bridge 模块：

```text
github.com/nexus-research-lab/nexus-agent-sdk-bridge
```

该模块提供 Nexus 需要共享的 client、protocol、permission、hook 和 MCP 契约。默认开源构建不依赖私有 runtime SDK。

检查依赖是否可解析：

```bash
make check-bridge-sdk-access
```

开发 bridge 时，可以临时指向本地仓库：

```bash
go mod edit -replace github.com/nexus-research-lab/nexus-agent-sdk-bridge=/path/to/nexus-agent-sdk-bridge
```

提交到 `main` 前，应恢复为公开模块版本：

```bash
go mod edit -dropreplace github.com/nexus-research-lab/nexus-agent-sdk-bridge
go mod tidy
```
