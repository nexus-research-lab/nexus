# Nexus Documentation

This directory contains documentation for users, operators, integrators, and
contributors. 中文文档为当前主要维护版本；英文内容会在对外入口和关键指南中逐步补齐。

The repository documents current behavior only. Historical implementation
plans, one-off audits, patent drafts, and local worktree notes do not belong in
the public documentation tree.

## Start here

| Topic | Document |
| --- | --- |
| Product overview | [English README](../README.md) · [中文 README](../README_zh.md) |
| Technical architecture | [Nexus 技术架构](./nexus-architecture-blueprint.md) |
| Linux production isolation | [Linux Runtime 隔离运维](./operations/runtime-isolation.md) |
| OpenAI Responses runtime | [OpenAI Responses runtime 集成](./specs/openai-responses-runtime-spec.md) |
| Prompt caching design note | [How Nexus Learns to Remember](./articles/nexus-prompt-cache.md) |

## Maintainer specifications

These documents describe current product contracts for contributors. They are
not a separately versioned public HTTP API. Code and tests remain authoritative
when a document and the current implementation disagree.

### Runtime, state, and security

- [Workspace 隔离与多用户运行时规范](./specs/workspace-isolation-spec.md)
- [Runtime 人工交互规范](./specs/permission-runtime-spec.md)
- [消息处理规范](./specs/message-processing-spec.md)
- [Session Key 统一规范](./specs/session-key-spec.md)
- [主智能体规范](./specs/main-agent-spec.md)
- [Slash 指令统一协议](./specs/slash-command-spec.md)

### Collaboration and execution

- [Room 模块规范](./specs/room-spec.md)
- [Room 协作协议](./specs/room-collaboration-spec.md)
- [Room Skill 编写指南](./specs/room-collaboration-mechanism.md) · [English](./specs/room-collaboration-mechanism.en.md)
- [Execution Orchestration 协议](./specs/execution-orchestration-spec.md)
- [Execution Graph 协议](./specs/execution-graph-spec.md)

### Platform capabilities

- [Nexus Skill 模型与运行时规范](./specs/skill-spec.md)
- [Connector OAuth Spec](./specs/connector-oauth-spec.md)
- [Scheduled Automation Permission Pipeline](./automation-permission-pipeline.md)
- [Nexus 对话配置控制面](./conversational-configuration-control.md)

## API status

The `/nexus/v1` HTTP and WebSocket routes are an application contract between
the Nexus backend, web client, and desktop hosts. They are not currently
published as a stable third-party API. The live route registry in
[`internal/app/server/routes.go`](../internal/app/server/routes.go) is the source
of truth; avoid copying it into a hand-maintained endpoint catalog.

## Documentation rules

- Describe behavior that exists on the default branch.
- Mark deployment prerequisites and security boundaries explicitly.
- Link to source files instead of copying volatile inventories.
- Keep proposals, migration scratchpads, and review notes in issues or pull
  requests rather than the public documentation tree.
