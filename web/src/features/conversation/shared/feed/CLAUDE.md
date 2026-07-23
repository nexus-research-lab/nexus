# feed/

L4 | 父级: web/src/features/conversation/shared

## 职责

- `conversation-feed.tsx`: DM 静态/虚拟消息流入口
- `conversation-virtual-feed.tsx`: 虚拟列表装配
- `conversation-round.tsx`: 静态与虚拟分支共用轮次渲染
- `conversation-feed-model.ts`: refs、renderer、source 与轮次状态投影
- `use-conversation-virtual-metrics.ts`: 容器宽度和导航偏移测量
- `use-conversation-round-navigation.ts`: 静态与虚拟列表共用轮次导航

高度估算必须同时依赖轮次身份、消息分组和容器宽度；不得用 ref 或仅用数组长度规避 Hook 依赖。
DM 与 Room、静态与虚拟 Feed 必须复用 `conversation-panel-styles.ts` 的内容轨道宽度，保证超宽屏扩展和虚拟高度估算使用同一真实容器。
