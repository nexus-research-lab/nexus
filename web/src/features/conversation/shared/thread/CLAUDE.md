# 共享 Thread 面板

本目录只负责 DM、Room 和子智能体可复用的 Thread 消息轨道与布局。

## 职责边界

- `conversation-thread-model.ts` 统一轮次、最后一轮权限、导航模式、展示模式和 Thread 身份投影。
- `conversation-thread-panel.tsx` 只组合纯模型与跟随滚动状态，不包含消息布局。
- `conversation-thread-view.tsx` 负责头部、消息轮次、滚动按钮和插槽渲染，不解释来源差异；移动布局必须复用平台感知的窄窗 Header 高度/gutter、拖窗热区、语义排版与 `UiIconButton`，桌面布局继续使用紧凑 Workspace Panel 几何。
- 上游负责提供已过滤的消息、轮次、身份和能力动作；本目录不得调用领域 API。
- Room 与子智能体不得复制 Thread 面板结构或从对方的私有目录反向导入。
- `transcript` 保留完整轮次、内部身份头和过程时间轴；`inspector` 由外层 Header 独占身份，只显示无左侧线点装饰的执行过程，并隐藏主 Feed 已经承载的用户输入与最终答复。
