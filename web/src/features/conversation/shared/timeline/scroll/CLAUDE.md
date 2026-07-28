# Conversation Timeline Scroll

- `use-follow-scroll.ts` 只编排 FOLLOW、READING、内容变化和滚动资源：FOLLOW 不读取可见锚点，READING 不调用贴底执行器，两者只能由用户滚动意图、显式回到底部或会话切换转换。
- `scroll-animation.ts` 独占 FOLLOW 与显式回到底部的 `scrollTop` 写入：普通内容增长在 layout effect / ResizeObserver 中同步写入真实 bottom，不创建 RAF；只有用户触发的回到底部保留 smooth 阻尼事务，初始化 `auto` 允许下一帧为虚拟测高再次收口。
- `history-prepend-anchor.ts` 管理历史前插的一次性锚点事务，取消、失败和会话切换必须清理快照。
- `conversation-viewport-anchor.ts` 按稳定 round 身份持续记录首个可见轮次；节点拓扑变化或静态/虚拟 Feed 切换后重新寻找同一节点并补偿视口，普通虚拟项测高仍由 Virtualizer 补偿。
- `use-follow-scroll-interactions.ts` 只把滚轮、pointer、触摸、键盘和原生滚动转换为跟随意图。
- `follow-scroll-model.ts` 保存实际滚动溢出、真实 bottom 判定与三类版本投影：`contentKey` 覆盖流式正文增长，`topologyKey` 覆盖消息/slot 以及精确 `agent_id + agent_round_id` 的 permission-first 节点身份增删与移动，`atomicLayoutKey` 覆盖权限模块和终态组件切换；版本只触发当前意图的重放，不得自行切换 FOLLOW/READING。
- 内容版本必须覆盖并行 Agent 的非末尾流式正文增长。Feed ResizeObserver 只拥有正文高度变化并随 topology 变化重新绑定；独立的滚动容器 ResizeObserver 处理 Composer、虚拟键盘及 App/浏览器窗口造成的 viewport 高度变化。两类变化都保持既有意图：FOLLOW 同步贴新 bottom，READING 恢复可见锚点。横向重排继续交给 Feed/Virtualizer。
- 用户已向上脱离 FOLLOW 时，Room 权限模块、终态正文与新成员回复都只恢复当前阅读锚点；仍在 FOLLOW 时，任何 chunk、终态或问题布局提交都在 paint 前直接贴真实 bottom，不能先恢复锚点再用弹簧追赶。
- 用户向上滚动的暂停意图必须粘住；只有明确向下回到底部、点击回到底部或切换会话才恢复跟随，各输入路径不得在面板投影中丢失。
- 回到底部入口只占自身浮动命中区；显隐不得调整 viewport，也不得用全宽层覆盖正文、Composer 或 Thread 分割线。
- `round-scroll.ts` 保存轮次 DOM 定位和导航目标协议，feed 与 navigator 共用同一实现。
- `use-scroll-anchored-state.ts` 只用于局部内容展开收起时保持可视位置，不参与消息历史前插。
