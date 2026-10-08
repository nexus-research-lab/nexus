# Nexus 文档

中文是主要维护语言。规范（specs）只描述已实现行为，文档与实现不一致时以代码和测试为准；explorations 是 non-normative 的在研资料，不能当作已交付行为引用。

## 入口

| 主题 | 文档 |
| --- | --- |
| 产品概览 | [中文 README](../README_zh.md) · [英文 README](../README.md) |
| 技术架构 | [Nexus 技术架构](./nexus-architecture-blueprint.md) · [架构图](./architecture-html/) |
| 模块边界 | [Internal 模块边界](./specs/internal-boundaries.md) |
| 前端 | [工程与设计系统治理](./specs/frontend-engineering-spec.md) · [视觉与交互](../design.md) · [Agent 入口](../web/AGENTS.md) |
| 权限与沙箱 | [帮我批准](./auto-review.md) · [桌面沙箱与一次性审批](./guides/desktop-sandbox.md) |
| Room Skill 编写 | [中文](./guides/room-skill-authoring.md) · [英文](./guides/room-skill-authoring.en.md) |
| WorkGraph | [设计原则](./guides/workgraph-design-principles.zh-CN.md) · [实现方案](./guides/workgraph-implementation-design.zh-CN.md) · [代码解读](./guides/workgraph-code-reading-report.zh-CN.md) · [方法论模板](./guides/workgraph-methodology-templates.zh-CN.md) |
| 运维 | [Linux Runtime 隔离](./operations/runtime-isolation.md) · [Control 部署与账号迁移](./operations/control-migration.md) |
| 回归测试 | [回归测试总目录](./testing/nexus-regression-catalog.md) · [桌面沙箱验收](./testing/desktop-sandbox-acceptance.md) · [IM 投递回传验收](./testing/im-delivery-replies.md) · [Provider 模型资料核对](./testing/provider-model-evidence.md) |

## 规范

### 运行时、状态与安全

- [工作区隔离与多用户运行时](./specs/workspace-isolation-spec.md)
- [运行时人工交互](./specs/permission-runtime-spec.md)
- [桌面沙箱](./specs/desktop-sandbox-spec.md)
- [消息处理](./specs/message-processing-spec.md)
- [失败解释与恢复](./specs/failure-recovery-spec.md)
- [Session Key](./specs/session-key-spec.md)
- [主智能体](./specs/main-agent-spec.md)
- [Slash 指令](./specs/slash-command-spec.md)
- [OpenAI Responses runtime](./specs/openai-responses-runtime-spec.md)
- [Provider 模型指引](./specs/provider-model-guidance-spec.md)

### 协作与执行

- [Agent 平台通讯](./specs/platform-communication-spec.md)
- [在线账号、Team 与 Node](./specs/online-team-spec.md)
- [Room 模块](./specs/room-spec.md) · [Room 协作协议](./specs/room-collaboration-spec.md)
- [执行编排](./specs/execution-orchestration-spec.md) · [执行图](./specs/execution-graph-spec.md)

### 平台能力与 Web

- [Skill 模型与运行时](./specs/skill-spec.md)
- [Connector OAuth](./specs/connector-oauth-spec.md)
- [定时自动化权限](./specs/automation-permission-pipeline-spec.md)
- [Echo 主动跟进](./specs/echo-spec.md)
- [Browser 能力](./specs/browser-spec.md)
- [对话配置控制面](./specs/conversational-configuration-control-spec.md)
- [Web 弹窗](./specs/dialog-design-spec.md) · [页面信息层级](./specs/web-surface-density-spec.md) · [能力页面](./specs/capability-page-design-spec.md)

## 在研资料（non-normative）

- [桌面沙箱：现状评估、Codex 对照、开发计划与验收](./explorations/desktop-sandbox/README.md)
- [WorkGraph 模板参数化与 Room 自适应](./explorations/workgraph-template-adaptation/README.md)
- [IM 投递来源记录与按需回传（历史提案）](./explorations/im-delivery-replies.md)
- [Go 后端重复与防御性代码治理审计](./explorations/backend-duplication-governance.zh-CN.md)

## API

`/nexus/v1` 下的 HTTP 与 WebSocket 路由服务 Nexus 后端、Web 客户端和桌面宿主，不是稳定的第三方 API。路由真相源是 [`internal/app/server/routes.go`](../internal/app/server/routes.go)，不另维护端点清单。

## 维护规则

- 规范只写已实现行为，注明部署前提与安全边界；链接代码真相源，不复制会漂移的清单。
- 一条规则只写在一处，其他文档链接过去，不转述。
- 提案、迁移草稿和评审记录放在 issue 或 PR；已授权的在研专题放 explorations 并标注 non-normative。
