# Conversation Timeline Scroll

- `use-follow-scroll.ts` 只编排 FOLLOW、READING、live 高度保护、内容变化和滚动资源：FOLLOW 的全部正向增长都按父 Feed 聚合后的真实高度贴底，live epoch 的负向校正暂存为父 Feed 高度负债；READING 永不调用贴底执行器。
- `use-conversation-live-height-guard.ts` 在单个会话 live epoch 内维护单调不减的 Feed 最小高度，并让静态内容栈与完整虚拟画布贴住底部，使负向估高/shell 收口负债只留在消息上方；正文与工具块仍按真实 DOM 自然增长，所有 live source 终态且异步终态布局经过短暂安静窗口后才一次性平滑释放，会话切换必须立即清空。
- `scroll-animation.ts` 独占共享 FOLLOW 与显式回到底部的 `scrollTop` 写入：静态尾部增长在 layout effect / ResizeObserver 中同步写入真实 bottom，不创建 RAF；用户触发的回到底部只对点击时已有距离保留 smooth 阻尼，后续内容高度提交必须取消该事务并把真实 bottom 交还 FOLLOW。初始化/新拓扑的 `auto` 必须保留到虚拟测高连续稳定后再交权。
- `history-prepend-anchor.ts` 管理历史前插的一次性锚点事务，取消、失败和会话切换必须清理快照。
- `conversation-viewport-anchor.ts` 按稳定 round 身份持续记录首个可见轮次；节点拓扑变化或静态/虚拟 Feed 切换后重新寻找同一节点并补偿视口，普通虚拟项测高仍由 Virtualizer 补偿。
- `use-follow-scroll-interactions.ts` 只把滚轮、pointer、触摸、键盘和原生滚动转换为跟随意图。
- `follow-scroll-model.ts` 保存 live layout epoch、实际滚动溢出、真实 bottom 判定、测高写入所有权与三类版本投影：`contentKey` 覆盖流式正文增长，`topologyKey` 覆盖消息/slot 以及精确 `agent_id + agent_round_id` 的 permission-first 节点身份增删与移动，`atomicLayoutKey` 覆盖权限模块和终态组件切换。
- 内容版本必须覆盖并行 Agent 的非末尾流式正文增长。静态 Feed 只观察父内容轨道，让同一布局周期内多个 Agent 的高度变化合并为一次真实 bottom 写入；虚拟 Feed 把普通 item 测高交给 Virtualizer；独立的滚动容器 ResizeObserver 处理 Composer、虚拟键盘及 App/浏览器窗口造成的 viewport 高度变化。
- 用户已向上脱离 FOLLOW 时，Room 权限模块、终态正文与新成员回复都只恢复当前阅读锚点；仍在 FOLLOW 时，任何 Agent 的增长都属于同一个 live timeline，必须保持真实 bottom。共享 bottom 事务、Virtualizer 与浏览器 clamp 任一时刻只能有一个 `scrollTop` 写入者；测量期间短暂离开数值 bottom 不得替用户切换到 READING。
- 用户向上滚动的暂停意图必须粘住；只有明确向下回到底部、点击回到底部或切换会话才恢复跟随，各输入路径不得在面板投影中丢失。
- 主 Feed 与 Thread 视口必须保留稳定 scrollbar gutter；滚动条出现或消失不得改变 Markdown 内容列宽并触发整段重新换行。
- 回到底部入口只占自身浮动命中区；显隐不得调整 viewport，也不得用全宽层覆盖正文、Composer 或 Thread 分割线。
- `round-scroll.ts` 保存轮次 DOM 定位和导航目标协议，feed 与 navigator 共用同一实现。
- `use-scroll-anchored-state.ts` 只用于局部内容展开收起时保持可视位置，不参与消息历史前插。
