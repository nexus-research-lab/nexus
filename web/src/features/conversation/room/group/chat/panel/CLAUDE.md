# Group Chat Panel

## 分层

- `group-chat-panel.tsx`：客户端入口，只组合模型与视图。
- `controller/`：按会话、输入、Goal 和纯投影阶段装配视图模型。
- `view/`：只渲染页面模型和负责人选择控件。
- `shared/session/use-conversation-session.ts`：统一管理 DM / Room 会话、历史、时间线与滚动。

## 约束

- 视图接口由 `view/group-chat-panel-view.tsx` 定义，控制器返回该具体模型。
- 会话事件、附件、Goal 和 Feed 逻辑不得回填到入口组件。
- Room 当前始终拥有会话控制权；不要重新引入恒为真的控制标记或空只读原因。
