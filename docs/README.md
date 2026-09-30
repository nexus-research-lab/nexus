# Nexus 文档

- [桌面沙箱与一次性审批](guides/desktop-sandbox.md)

本目录收录面向用户、运维人员、集成开发者和贡献者的文档。中文是当前主要维护语言，对外入口和关键指南会逐步提供英文版本。

产品指南与维护者规范记录当前行为。明确授权保留的在研改造资料集中在 explorations，标注 non-normative，并与当前规范分开；其历史证据不得作为已交付行为引用。

## 文档入口

| 主题 | 文档 |
| --- | --- |
| 产品概览 | [中文 README](../README_zh.md) · [英文 README](../README.md) |
| 技术架构 | [Nexus 技术架构](./nexus-architecture-blueprint.md) |
| 前端代码与 UI 规范 | [工程与设计系统治理](./specs/frontend-engineering-spec.md) · [视觉与交互唯一入口](../design.md) · [Agent 入口](../web/AGENTS.md) |
| Room Skill 编写 | [中文指南](./guides/room-skill-authoring.md) · [英文指南](./guides/room-skill-authoring.en.md) |
| WorkGraph 设计理念 | [问题定义与设计原则](./guides/workgraph-design-principles.zh-CN.md) |
| WorkGraph 实现方案 | [实现流程与使用说明](./guides/workgraph-implementation-design.zh-CN.md) |
| WorkGraph 代码解读 | [代码依据与保护边界](./guides/workgraph-code-reading-report.zh-CN.md) |
| WorkGraph 方法论模板 | [模板分类与产物契约](./guides/workgrpah/workgraph-methodology-templates.zh-CN.md) |
| Linux 生产隔离 | [Linux Runtime 隔离运维](./operations/runtime-isolation.md) |
| Control 部署与账号迁移 | [Nexus Control 部署与迁移](./operations/control-migration.md) |
| OpenAI Responses runtime | [OpenAI Responses runtime 集成](./specs/openai-responses-runtime-spec.md) |
| Echo 主动跟进 | [Echo 主动跟进模块](./specs/echo-spec.md) |
| 维护者回归测试 | [Nexus 回归测试总目录](./testing/nexus-regression-catalog.md) · [会话打开与最新消息滚动](./testing/conversation-latest-scroll-regression.md) |

## 维护者规范

这些文档记录贡献者需要遵守的当前产品合同，不单独作为公共 HTTP API 进行版本管理。文档与当前实现不一致时，以代码和测试为准。

### 运行时、状态与安全

- [工作区隔离与多用户运行时规范](./specs/workspace-isolation-spec.md)
- [运行时人工交互规范](./specs/permission-runtime-spec.md)
- [桌面沙箱当前规范](./specs/desktop-sandbox-spec.md)
- [消息处理规范](./specs/message-processing-spec.md)
- [失败解释与恢复基础协议](./specs/failure-recovery-spec.md)
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
- [Web 页面信息层级规范](./specs/web-surface-density-spec.md)
- [Nexus Skill 模型与运行时规范](./specs/skill-spec.md)
- [Connector OAuth 规范](./specs/connector-oauth-spec.md)
- [定时自动化权限规范](./specs/automation-permission-pipeline-spec.md)
- [Echo 主动跟进模块规范](./specs/echo-spec.md)
- [Browser 能力规范](./specs/browser-spec.md)
- [Nexus 对话配置控制面](./specs/conversational-configuration-control-spec.md)

## API 状态

`/nexus/v1` 下的 HTTP 与 WebSocket 路由用于连接 Nexus 后端、Web 客户端和桌面宿主，当前没有作为稳定的第三方 API 发布。路由真相源位于 [`internal/app/server/routes.go`](../internal/app/server/routes.go)，不要另行维护容易漂移的端点清单。

## 在研改造资料

- [桌面沙箱：现状评估、Codex 对照、开发计划与验收](./explorations/desktop-sandbox/README.md)（non-normative）。
## IM 投递回传

- [当前通讯合同](specs/platform-communication-spec.md)
- [本地与真实通道验收](testing/im-delivery-replies.md)


## 文档维护规则

- 产品指南与当前规范只描述已实现行为，并明确分支、版本与部署前提。
- 明确标注部署前置条件和安全边界。
- 链接到代码真相源，避免复制容易漂移的清单。
- 一般提案、迁移草稿和评审记录放在 issue 或 pull request 中；已授权的在研专题通过独立入口维护，不混入当前规范。
