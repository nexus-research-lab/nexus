# 当前功能事实来源

这份文件只供维护和核验手册使用，不直接展示给普通用户。

## 核验顺序

1. 先确认功能已经接入当前界面或当前运行服务。
2. 再用 `docs/specs/` 中标为当前状态的规范确认行为和限制。
3. 用界面翻译文件核对用户实际看到的名称。
4. 设计草案、未来规划、未挂载组件和测试占位不能写成当前功能。
5. 如果代码、规范和界面名称不一致，在修正产品前采用最保守的说法，并明确当前限制。

## 来源地图

| 功能 | 主要事实来源 |
| --- | --- |
| 页面与入口 | `web/src/app/router/route-paths.ts`、`web/src/shared/i18n/catalog/zh/navigation.ts` |
| 启动页和首次设置 | `web/src/features/launcher/`、`web/src/features/onboarding/` |
| 左侧能力目录 | `web/src/features/capability/sidebar/capability-sidebar-model.ts` |
| 会话标签和固定会话 | `web/src/shared/ui/workspace/controls/`、`web/src/store/room-navigation.ts` |
| 会话历史 | `web/src/features/conversation/room/surface/history/` |
| 消息和输入框 | `web/src/features/conversation/`、`docs/specs/message-processing-spec.md`、`docs/specs/slash-command-spec.md` |
| 智能体详情 | `web/src/features/agents/`、`web/src/features/contacts/` |
| 记忆 | `web/src/features/memory/CLAUDE.md` |
| 好友联络 | `docs/specs/platform-communication-spec.md`、`web/src/features/contacts/CLAUDE.md` |
| 房间协作 | `docs/specs/room-spec.md`、`web/src/features/conversation/room/` |
| Goal 和工作图 | `docs/specs/execution-graph-spec.md`、`docs/specs/execution-orchestration-spec.md`、`docs/specs/slash-command-spec.md`、`web/src/features/conversation/shared/execution/` |
| 工作图模板 | `docs/specs/execution-graph-spec.md`、`web/src/features/capability/workgraph-distillations/` |
| 主动跟进 | `docs/specs/echo-spec.md`、`web/src/features/settings/general/` |
| 定时任务 | `docs/specs/automation-permission-pipeline-spec.md`、`web/src/features/capability/scheduled/` |
| 浏览器 | `docs/specs/browser-spec.md`、`desktop/browser-extension/README.md`、`web/src/features/settings/browser/` |
| Skill | `docs/specs/skill-spec.md`、`web/src/features/capability/skills/` |
| 工作循环 | `web/src/features/capability/loops/` |
| 连接器 | `web/src/features/capability/connectors/`、`docs/specs/connector-oauth-spec.md` |
| 消息通道与配对 | `web/src/features/capability/channels/`、`web/src/features/capability/channels/pairings/` |
| 设置和模型服务 | `web/src/features/settings/`、`web/src/features/settings/provider-settings/` |
| 管理后台 | `web/src/features/settings/operations/` |

## 文案检查

- 每项功能至少说明：用途、入口、操作、结果、当前限制。
- 入口必须使用当前界面名称。
- 不把内部默认值写成用户承诺；时间、次数、自动关闭等策略应使用保守描述。
- 不把本机保存状态说成账号同步，也不把网页端能力说成桌面端能力。
- “可以查看”不等于“可以编辑”；“已配置”不等于“当前会话已启用”。
