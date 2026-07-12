# Group Conversation Feed

## 数据流

`GroupConversationFeedProps` 按 `refs`、`source`、`renderer` 分组。普通列表与虚拟列表都通过 `resolveGroupConversationRound` 产生轮次状态，并由 `GroupConversationRound` 渲染。

## 约束

- 不在普通与虚拟列表中分别实现加载、Room 卡片或普通消息判断。
- Agent 身份目录、会话命令和运行阶段由面板投影完整提供，Feed 不维护假可选契约或空对象兼容分支。
- 导航优先定位已挂载 DOM；虚拟列表未挂载时才回退到索引滚动。
- 模型文件只做纯数据转换，不读取 Store、不触发副作用。
