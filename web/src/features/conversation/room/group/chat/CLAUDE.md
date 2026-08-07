# Room Group Chat

## 职责

- `panel/controller/` 按会话、输入、Goal 和视图投影阶段组合 Room 会话。
- `panel/view/` 只渲染面板布局与局部控件。
- `feed/` 把轮次源渲染为普通或虚拟列表。
- 根目录只保留空状态和 Goal 等 Chat 直属能力；Thread 实时桥归相邻 `group/thread/live/`。

## 边界

- 面板不得自行实现消息轮次分支；轮次展示统一进入 `feed/`。
- Chat 控制器只向 Thread 发布最小实时源，不读取 Thread 私有 Store。
- 新增控制状态前先确认它属于会话、Feed、Goal 还是 Thread，不在入口组件堆叠。
- Room 输入框提供“全部停止当前输出”聚合动作；点击时冻结 execution state 与 pending slot 中仍 active、且尚未 stopping 的精确 `agent_round_id` 集合，再逐个复用定向停止，不能发送无目标的 DM/session interrupt。
