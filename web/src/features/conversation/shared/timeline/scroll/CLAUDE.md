# Conversation Timeline Scroll

- `use-follow-scroll.ts` 只编排跟随状态、内容变化和滚动资源，不实现动画算法或手势细节。
- `scroll-animation.ts` 独占平滑滚动 RAF 生命周期；任何新滚动开始前都必须取消旧动画。
- `history-prepend-anchor.ts` 管理历史前插的一次性锚点事务，取消、失败和会话切换必须清理快照。
- `use-follow-scroll-interactions.ts` 只把滚轮、触摸和原生滚动转换为跟随意图。
- `follow-scroll-model.ts` 保存底部判定与内容身份纯模型，DM、Room 和 Thread 不得重复推导。
- `round-scroll.ts` 保存轮次 DOM 定位和导航目标协议，feed 与 navigator 共用同一实现。
- `use-scroll-anchored-state.ts` 只用于局部内容展开收起时保持可视位置，不参与消息历史前插。
