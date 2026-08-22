# Nexus 文档

本目录收录面向用户、运维人员、集成开发者和贡献者的文档。中文是当前主要维护语言，对外入口和关键指南会逐步提供英文版本。

仓库只记录当前行为。历史实施计划、一次性审计、专利草稿和本地 worktree 记录不进入公开文档目录。

## 文档入口

| 主题 | 文档 |
| --- | --- |
| 产品概览 | [中文 README](../README_zh.md) · [英文 README](../README.md) |
| 技术架构 | [Nexus 技术架构](./nexus-architecture-blueprint.md) |
| Room Skill 编写 | [中文指南](./guides/room-skill-authoring.md) · [英文指南](./guides/room-skill-authoring.en.md) |
| Linux 生产隔离 | [Linux Runtime 隔离运维](./operations/runtime-isolation.md) |
| OpenAI Responses runtime | [OpenAI Responses runtime 集成](./specs/openai-responses-runtime-spec.md) |
| Echo 主动跟进 | [Echo 主动跟进模块](./specs/echo-spec.md) |
| 维护者回归测试 | [Nexus 回归测试总目录](./testing/nexus-regression-catalog.md) · [会话打开与最新消息滚动](./testing/conversation-latest-scroll-regression.md) |

## 维护者规范

这些文档记录贡献者需要遵守的当前产品合同，不单独作为公共 HTTP API 进行版本管理。文档与当前实现不一致时，以代码和测试为准。

### 运行时、状态与安全

- [工作区隔离与多用户运行时规范](./specs/workspace-isolation-spec.md)
- [运行时人工交互规范](./specs/permission-runtime-spec.md)
- [消息处理规范](./specs/message-processing-spec.md)
- [Session Key 统一规范](./specs/session-key-spec.md)
- [主智能体规范](./specs/main-agent-spec.md)
- [Slash 指令统一协议](./specs/slash-command-spec.md)

### 协作与执行

- [Agent 平台通讯规范](./specs/platform-communication-spec.md)
- [Room 模块规范](./specs/room-spec.md)
- [Room 协作协议](./specs/room-collaboration-spec.md)
- [执行编排协议](./specs/execution-orchestration-spec.md)
- [执行图协议](./specs/execution-graph-spec.md)

### 平台能力

- [Web 弹窗设计规范](./specs/dialog-design-spec.md)
- [Nexus Skill 模型与运行时规范](./specs/skill-spec.md)
- [Connector OAuth 规范](./specs/connector-oauth-spec.md)
- [定时自动化权限规范](./specs/automation-permission-pipeline-spec.md)
- [Echo 主动跟进模块规范](./specs/echo-spec.md)
- [Browser 能力规范](./specs/browser-spec.md)
- [Nexus 对话配置控制面](./specs/conversational-configuration-control-spec.md)

## API 状态

`/nexus/v1` 下的 HTTP 与 WebSocket 路由用于连接 Nexus 后端、Web 客户端和桌面宿主，当前没有作为稳定的第三方 API 发布。路由真相源位于 [`internal/app/server/routes.go`](../internal/app/server/routes.go)，不要另行维护容易漂移的端点清单。

## 文档维护规则

- 只描述默认分支已经存在的行为。
- 明确标注部署前置条件和安全边界。
- 链接到代码真相源，避免复制容易漂移的清单。
- 提案、迁移草稿和评审记录放在 issue 或 pull request 中，不进入公开文档目录。
