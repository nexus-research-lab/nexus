# Conversation Shared

跨 DM 与 Room 复用的对话基础设施。

## 根模块

- `conversation-panel-layout.tsx`：会话面板通用布局和浮层控件。
- `conversation-panel-styles.ts`：统一消息流、助手正文、告警与 Composer 的桌面宽度阶梯。
- `conversation-panel-model.ts`：把共享会话控制器和面板环境投影为 Frame、导航、视口和滚动控件模型。
- `use-conversation-panel-environment.ts`：统一读取用户头像、布局模式和 Provider 告警状态。
- `use-conversation-snapshot-reporter.ts`：按会话作用域报告稳定快照，并统一活跃时间、当前已加载消息计数与显式 `has_user_input` 投影；只有可见且非 synthetic 的 user 消息属于用户输入，消息计数不得代替该事实。Conversation scope 切换后的首个 effect 必须跳过，且消息 identity 必须属于当前 scope，防止上一会话的消息集合在清空前污染新 draft。
- `conversation-error-bubble.tsx`：按有序诊断规则投影用户可执行的错误说明并渲染统一错误消息。

## 约束

- 共享层只承载 DM 与 Room 语义完全一致的结构，不吸收各领域的差异字段。
- 纯投影不得持有 React 状态或调用领域 API。
- 错误分类按具体 Provider、实时连接、通用后端的顺序匹配，不在视图中追加条件分支。
- 具体 Feed、Goal 和 Composer 模型由各自领域定义。
- 常规桌面保持紧凑阅读宽度；超宽屏只按共享阶梯放宽消息轨道和 Composer，助手正文使用更小的可读上限，禁止各消费面自行复制宽度断点。
- 回到底部与当前会话 Activity 共享绝对锚在 Composer 顶边的居中 Dock：Task 存在时占中心主位，回到底部作为同一行相邻圆形动作；Task 缺席时回到底部单独居中。禁止把 Goal 与 Composer 的组合容器当作坐标原点。Goal/告警留在 pre-composer slot；该 slot 有内容时以透明 runway 隔开 Dock 与状态内容。Dock 容器和中间包装不得接收指针，只有 Task 与回到底部的真实按钮热区接收；仅在控件真实可见时，阅读 viewport 尾部才保留 56px 避让，隐藏时不得制造空白。控件显隐与 Task 展开不得改变 viewport 高度。
- Session 导航只能绝对定位在 `ConversationPanelViewportArea` 内，不得用 Composer 猜测高度设置固定 bottom；Goal、附件或多行输入改变底部区域时，导航可用高度必须自动跟随真实 viewport。
- 主消息 viewport 保留 `tabIndex={-1}` 供程序化导航，但显式移除浏览器原生轮廓；Safari 不得在滚动区底边绘制跨栏蓝线。
