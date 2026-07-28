# feed/

L4 | 父级: web/src/features/conversation/shared

## 职责

- `conversation-feed.tsx`: DM 静态/虚拟消息流入口
- `conversation-virtual-feed.tsx`: 虚拟列表装配
- `conversation-round.tsx`: 静态与虚拟分支共用轮次渲染
- `conversation-feed-model.ts`: refs、renderer、source 与轮次状态投影
- `use-conversation-virtual-metrics.ts`: 容器宽度和导航偏移测量
- `use-conversation-virtual-scroll-policy.ts`: 稳定轮次 key、静态 Feed 切换时的初始偏移快照与动态高度锚点策略
- `use-conversation-round-navigation.ts`: 静态与虚拟列表共用轮次导航

高度估算必须同时依赖轮次身份、消息分组和容器宽度；不得用 ref 或仅用数组长度规避 Hook 依赖。
DM 与 Room、静态与虚拟 Feed 必须复用 `conversation-panel-styles.ts` 的内容轨道宽度，保证超宽屏扩展和虚拟高度估算使用同一真实容器。
虚拟尺寸变化只补偿完整位于视口上方的轮次；横跨视口的流式长回复不得在用户暂停跟随后继续推走阅读位置。
静态 Feed 跨过虚拟化阈值时，Virtualizer 必须继承同一滚动容器的既有 `scrollTop`，不得以默认零偏移覆盖用户正在阅读的位置。
虚拟 Feed 根节点必须声明 `data-conversation-virtual-feed="true"`，避免共享滚动层重复执行非虚拟可见轮次补偿。
