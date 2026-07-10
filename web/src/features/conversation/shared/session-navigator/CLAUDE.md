# session-navigator/

L4 | 父级: web/src/features/conversation/shared

## 职责

- `conversation-session-navigator.tsx`: 会话刻度与预览面板渲染
- `session-navigator-model.ts`: 时间线到导航项的纯数据投影与刻度视觉模型
- `use-conversation-session-navigation.ts`: 可见轮次同步、缺失窗口加载与跳转编排

导航目标以滚动容器的 `data-conversation-round-navigation-target` 为唯一真相，
不要再增加平行 ref 或局部状态镜像。
