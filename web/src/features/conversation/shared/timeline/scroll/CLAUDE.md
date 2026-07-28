# Conversation Timeline Scroll

- `use-follow-scroll.ts` 只编排跟随状态、内容变化和滚动资源，不实现动画算法或手势细节。
- `scroll-animation.ts` 独占动态底部目标与阻尼跟随 RAF 生命周期；自动跟随在同一事务内只接受底部目标单向增长，忽略 ResizeObserver/虚拟测高的瞬时回缩，显式新定位仍按实时底部收口；活动跟随检测到阅读 viewport 高度变化时必须在写入 `scrollTop` 前终止。
- `history-prepend-anchor.ts` 管理历史前插的一次性锚点事务，取消、失败和会话切换必须清理快照。
- `conversation-viewport-anchor.ts` 按稳定 round 身份持续记录首个可见轮次；节点拓扑变化或静态/虚拟 Feed 切换后重新寻找同一节点并补偿视口，普通虚拟项测高仍由 Virtualizer 补偿。
- `use-follow-scroll-interactions.ts` 只把滚轮、pointer、触摸、键盘和原生滚动转换为跟随意图。
- `follow-scroll-model.ts` 保存实际滚动溢出、底部判定与三类版本投影：`contentKey` 覆盖流式正文增长，`topologyKey` 只覆盖消息/slot 节点身份的增删与移动，`atomicLayoutKey` 覆盖权限模块和终态组件切换；回到底部入口只有在容器确实可滚动时才可见，DM、Room 和 Thread 不得重复推导。
- 内容版本必须覆盖并行 Agent 的非末尾流式正文增长；虚拟列表的立即贴底要在下一帧测量完成后再次收口。Feed ResizeObserver 只拥有正文高度变化并随 topology 变化重新绑定；独立的滚动容器 ResizeObserver 处理 Composer、虚拟键盘及 App/浏览器窗口造成的 viewport 高度变化，保留原可见位置并按结果保持或解除跟随，绝不能把它误当成正文增长。横向重排继续交给 Feed/Virtualizer，不得仅因宽度变化解除跟随。
- 用户已向上脱离跟随时，Room 权限模块、终态正文与新成员回复都不得改写当前阅读位置；仍在底部时只允许单向向下追随，不能因临时测量回缩而先上后下。
- 一次提交造成的大块高度增长属于原子布局替换，不是流式追随目标；保留当前可见内容并露出回到底部入口，小幅真实 chunk 增长才继续跟随。
- 用户向上滚动的暂停意图必须粘住；只有明确向下回到底部、点击回到底部或切换会话才恢复跟随，各输入路径不得在面板投影中丢失。
- 回到底部入口占用 Feed 外恒定高度的安全带；按钮显隐不得调整 viewport，也不得覆盖正文、Composer 或 Thread 分割线。
- `round-scroll.ts` 保存轮次 DOM 定位和导航目标协议，feed 与 navigator 共用同一实现。
- `use-scroll-anchored-state.ts` 只用于局部内容展开收起时保持可视位置，不参与消息历史前插。
