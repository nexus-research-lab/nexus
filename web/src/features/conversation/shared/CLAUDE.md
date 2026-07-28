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
- 浮动回到底部动作复用共享暖色控制面、边界和阴影，不以高亮纯白圆块脱离工作区层级。
