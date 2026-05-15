<div align="center">

# Nexus

**本地部署的多智能体工作台**

把人和多个 AI 智能体组织到同一个工作流里，完整掌控运行状态、权限和工作区数据。

[![Go 1.26+](https://img.shields.io/badge/go-1.26+-00ADD8.svg)](https://go.dev/)
[![Node.js 22+](https://img.shields.io/badge/node-22+-339933.svg)](https://nodejs.org/)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-yellow.svg)](https://www.apache.org/licenses/LICENSE-2.0)

<p align="center">
  <strong>中文</strong> | <a href="./README.md">English</a>
</p>

</div>

---

<div align="center">
<img src="./docs/image/app.png" alt="Nexus 工作台" width="90%">
</div>

---

## 什么是 Nexus

Nexus 是一个可以自托管的多智能体工作台。你可以在**房间**里让多个 AI 智能体围绕同一个任务协作，也可以通过**直接会话**和单个智能体持续深入地工作。整个系统运行在你自己的机器或服务器上，数据不离开本地。

与单纯的对话界面不同，Nexus 提供了完整的运行时控制：你可以排队、中断、切换权限模式，能看到智能体正在做什么，也能随时介入。

---

## 核心能力

### 多智能体房间
在一个共享空间里协调多个智能体。@提及、请求回复、发起私域动作——多个智能体可以围绕同一个问题分工推进，结果汇聚到同一个会话线索里。

### 一对一直接会话
与单个智能体保持持续的工作关系。流式输出、消息排队、运行中断、历史回放，会话状态在关闭后仍然保留。

### 工作区隔离
每个智能体有独立的工作目录、技能配置和权限边界。用户、智能体、文件和连接器数据按归属严格隔离，不会互相干扰。

### 技能与自动化
为智能体安装内置技能（图片生成、记忆管理、房间协作规则等），设置定时任务和持续跟进任务，让智能体按计划持续推进工作。

---

## 快速开始

**下载发布包（推荐）**

```bash
# 解压（以 Linux x86_64 为例）
tar -xzf nexus-v0.1.3-linux-amd64.tar.gz
cd nexus-v0.1.3-linux-amd64

# 初始化数据库并创建管理员账号
./bin/nexus-migrate up
printf '%s\n' 'your-password' | ./bin/nexusctl auth init-owner --username admin --password-stdin

# 启动
./run-nexus
```

打开浏览器访问 `http://localhost:8010`，使用刚创建的账号登录。

<details>
<summary>Windows</summary>

```bat
bin\nexus-migrate.exe up
echo your-password| bin\nexusctl.exe auth init-owner --username admin --password-stdin
run-nexus.cmd
```

</details>

<details>
<summary>从源码启动（开发模式）</summary>

```bash
# 需要 Go 1.26+、Node.js 22+、pnpm 9.15+
make install
make dev
```

后端监听 `http://localhost:8010`，前端开发服务监听 `http://localhost:3000`。

</details>

---

## 核心概念

| 概念 | 说明 |
|------|------|
| **Agent（智能体）** | 独立的 AI 工作单元，有自己的模型配置、技能列表和工作区文件 |
| **Room（房间）** | 多个智能体和用户共享的协作空间，支持公开消息和私域动作 |
| **DM（直接会话）** | 与单个智能体的持续对话，保留完整的运行状态和历史 |
| **Workspace（工作区）** | 每个智能体独立的文件目录，支持浏览、上传、编辑和下载 |
| **Skill（技能）** | 安装到智能体的能力扩展，可以是内置技能或自定义技能 |
| **Connector（连接器）** | 用于管理 OAuth 应用配置和外部服务账号连接的模块 |

---

## 内置技能

Nexus 在 `skills/` 目录下提供一组开箱即用的技能：

| 技能 | 功能 |
|------|------|
| `imagegen` | 调用图片生成 Provider，将结果保存到智能体工作区 |
| `nexus-manager` | 让智能体通过命令操作 Nexus 的智能体、房间、会话和工作区 |
| `room-playbook` | 为房间协作提供固定规则和操作指引 |
| `scheduled-task-manager` | 管理定时任务与 heartbeat 类持续跟进任务 |
| `memory-manager` | 按约定维护项目记忆文件 |

---

## 功能全览

| 功能模块 | 说明 |
|----------|------|
| 多智能体房间 | 公开消息、私域动作、定向回复、延迟唤醒、房间级技能规则 |
| 直接会话 | 流式输出、消息排队、运行中断、历史记录、运行态恢复 |
| 权限控制 | 多种权限模式，保留 AskUserQuestion 类用户交互通道 |
| 工作区文件 | 浏览、上传、编辑、下载、重命名和删除智能体工作区文件 |
| 图片生成 | 支持独立图片生成 Provider，通过技能和 CLI 调用 |
| 定时任务 | 创建、编辑、手动运行定时任务，查看运行历史 |
| 连接器 | 管理 OAuth 应用配置和外部服务账号连接 |
| 单端口部署 | Go 服务直接托管前端，运行包解压即用，无需额外启动前端 |

---

## 发布包

每次正式发布提供以下平台的可运行包：

| 平台 | 格式 |
|------|------|
| `linux-amd64` | `.tar.gz` |
| `linux-arm64` | `.tar.gz` |
| `windows-amd64` | `.zip` |

每个包包含：服务端二进制（`nexus-server`、`nexus-migrate`、`nexusctl`）、数据库迁移脚本、内置技能和构建好的前端资源。

---

## 构建

**依赖环境**

- Go 1.26.2+
- Node.js 22+
- pnpm 9.15.2+

**常用命令**

| 命令 | 说明 |
|------|------|
| `make dev` | 同时启动后端和前端开发服务 |
| `make check` | 执行 Go 测试、前端 lint 和类型检查 |
| `make db-init` | 执行数据库迁移 |
| `make gen-protocol-types` | 根据 Go 协议模型重新生成前端类型 |
| `make package-release` | 为当前平台构建可运行包 |

**构建指定平台包**

```bash
NEXUS_RELEASE_TARGET=linux-amd64  ./scripts/package-release.sh 0.1.3
NEXUS_RELEASE_TARGET=linux-arm64  ./scripts/package-release.sh 0.1.3
NEXUS_RELEASE_TARGET=windows-amd64 ./scripts/package-release.sh 0.1.3
```

> Nexus 通过 CGO 依赖 SQLite。跨平台构建时需要安装目标平台的 C 工具链。

---

## Go Bridge SDK

本仓库依赖公开的 Go bridge 模块：

```
github.com/nexus-research-lab/nexus-agent-sdk-bridge
```

该模块提供客户端、协议、权限、hook 和 MCP 等共享契约。默认开源构建不依赖私有 runtime SDK。

```bash
# 检查依赖是否可解析
make check-bridge-sdk-access
```

---

## 许可证

Apache License 2.0，详见 [LICENSE](./LICENSE)。
