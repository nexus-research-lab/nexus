# Room Thread 实时数据

- `room-thread-live-store.ts` 保存当前会话发布的最小实时切片，不承担领域投影。
- `room-thread-panel-model.ts` 根据 Thread 目标纯派生面板模型。
- 模型显式接收当前翻译函数，由共享名称所有者处理缺失/空白 Agent 名称；hook 随语言与实时源更新。名称目录不完整不能改变 exact agent_round 的消息、权限或运行态切片。
- `use-room-thread-source.ts` 负责发布与清理会话切片，并在会话切换时关闭旧 Thread。
- 文件回调复用 `lib/workspace-file-action.ts` 的 `WorkspaceFileOpenHandler`，稳定桥接必须将 path 与可选 workspaceAgentId 一起交给最新 handler；不得丢失第二参数并触发当前工作区回退。
- `use-room-thread-panel.ts` 是桌面与移动端共用的消费入口。

实时源不保存可从上下文得到的会话标识；生产者不订阅自身发布的数据，避免反馈更新。Agent 身份映射由唯一生产者完整提供，消费端只处理具体成员缺失，不接受目录整体缺失。面板消息、权限与 execution 首见锚点按 Thread 目标的 `agent_round_id` 精确筛选；Room 停止不经过这里，而由主 Feed 的 slot 卡片直接发起。
