# Group Chat Panel

## 分层

- `group-chat-panel.tsx`：客户端入口，只组合模型与视图。
- `use-group-chat-panel-model.ts`：把页面输入装配为视图模型。
- `use-group-chat-session.ts`：管理会话、历史、时间线与滚动。
- `use-room-goal-composer.ts`：管理 Goal 负责人和创建动作。
- `group-chat-panel-view.tsx`：只渲染页面模型。

## 约束

- 视图接口由 `group-chat-panel-view.tsx` 定义，控制器返回该具体模型。
- 会话事件、附件、Goal 和 Feed 逻辑不得回填到入口组件。
- Room 当前始终拥有会话控制权；不要重新引入恒为真的控制标记或空只读原因。
