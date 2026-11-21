# feed/

L4 | 父级: web/src/features/conversation/shared

## 职责

- `conversation-feed.tsx`: DM 静态/虚拟消息流入口
- `conversation-virtual-feed.tsx`: 虚拟列表装配
- `conversation-round.tsx`: 静态与虚拟分支共用轮次渲染
- `conversation-feed-model.ts`: refs、renderer、source 与轮次状态投影
- `conversation-feed-tail.tsx`: 静态与虚拟 Feed 共用、不改变增长节奏的唯一底部锚点
- `conversation-virtual-canvas.tsx`: 从 Feed 顶部按当前真实总高度放置虚拟画布
- `use-conversation-virtual-metrics.ts`: viewport 宽度和导航偏移测量
- `use-conversation-virtual-scroll-policy.ts`: 稳定轮次 key、静态 Feed 切换时的初始偏移快照与阅读位置恢复、连续窗口拓扑变化时的虚拟阅读锚点、新挂载行的同步测高与动态高度锚点策略
- `use-conversation-virtualization-policy.ts`: 会话内锁存 static/virtual renderer，并只在 live epoch 结束后的稳定窗口跨越阈值
- `use-conversation-round-navigation.ts`: 静态与虚拟列表共用轮次导航

高度估算必须同时依赖轮次身份、消息分组和容器宽度；不得用 ref 或仅用数组长度规避 Hook 依赖。
DM 与 Room、静态与虚拟 Feed 必须复用 `conversation-panel-styles.ts` 的内容轨道宽度，保证超宽屏扩展和虚拟高度估算使用同一真实容器。
虚拟尺寸变化服从共享 scroll owner：bottom 或轮次导航事务活跃时 Virtualizer 不写 `scrollTop`；FOLLOW 中正负 measured delta 均由 Virtualizer 保持底部；READING 中完整位于视口上方的轮次始终补偿（包括用户直接滚动期间，否则向上加载的历史会把视口推回新窗口开头），补偿叠加到实时 scrollTop 而非上次观测的 offset；直接滚动 epoch 内横跨或位于视口下方的变化不得反向写回，横跨视口的流式长回复不得推走阅读位置。局部展开/收起不得在 Virtualizer 之外再补写同一尺寸 delta。
静态 Feed 只在 live epoch 结束后的稳定窗口跨过虚拟化阈值；Virtualizer 必须继承同一滚动容器的既有阅读位置（FOLLOW 继承 `scrollTop`，READING 按轮次身份继承首个可见轮次及其视口内偏移，切换与窗口前插落在同一次提交时下标已经变化），不得在并行输出中途替换 renderer 或以默认零偏移覆盖用户正在阅读的位置。
`leadingContent` 只承载不属于 transcript 的本地视图前导，并必须进入静态 Feed 根节点，与消息轮次共同按真实高度布局并接受调用方指定的初始滚动锚点；只有前导且没有消息轮次时静态 Feed 必须至少填满 viewport，让欢迎页可按真实可见区居中，嵌入式顶部接待说明仍保持自身起始对齐；存在该前导时不得切换为未计入其高度的虚拟画布，禁止把前导放在 Feed 外制造两段内容栈。
活跃轮次的虚拟估高不能消费尚未 reveal 的完整 backend target；先使用保守基线，再由真实 Markdown/工具 DOM 的正向 measured delta 驱动增长。终态历史仍按完整内容估高。
Feed 只挂载正文已驻留的连续窗口，索引中存在但未驻留的轮次不占滚动空间、不渲染占位节点；窗口在两端由连续窗口 loader 扩展。窗口前插或头部淘汰改变 item 下标时，Virtualizer 按稳定节点身份把同一轮次保持在原视口位置，每次测高只补偿该次 delta（TanStack 累加的 adjustments 不得重复叠加到实时 `scrollTop`）。起点淘汰让画布变短、浏览器先把 `scrollTop` 夹到新的最大值时，拓扑位移以提交前的位置为基准，不得把被夹掉的距离再扣一次。拓扑提交期间 `scrollTop` 仍属于旧下标，测高补偿改按阅读锚点下标判断（只补偿锚点之前的项）；新挂载的行在同一次 layout commit 内测完，不等滚动结束后的 ResizeObserver，避免先画出一帧估高排布。静态 Feed 切换为虚拟 Feed 时，阅读中的初始偏移取切换前首个可见轮次的估高起点（静态 `scrollTop` 按真实高度累计，直接继承会让该轮次落在按估高计算的首个渲染区间之外），首批行挂载后再由同一 Hook 按真实 DOM 位置把它恢复到原视口位置；虚拟 Feed 内 DOM 阅读锚点不写 `scrollTop`。
轮间距必须放在每个可测量 round wrapper 内，静态父容器不得使用 `gap`/`space-y`；否则跨过虚拟化阈值时累计间距会消失并造成整屏位移。
虚拟 Feed 根节点必须声明 `data-conversation-virtual-feed="true"`，避免共享滚动层重复执行可见轮次补偿或 bottom 写入。
最后一个 root 不得预建 viewport 高度的空白 runway。新消息从真实内容底部进入；FOLLOW 把并行 Agent 视为同一条 live timeline，父 Feed 聚合尺寸变化后始终贴住真实底部，让旧内容连续上推。Feed 必须始终使用当前真实内容高度；过程区自动或手动收起、终态切换和虚拟估高校正不得保留历史最大 min-height，也不得通过底部对齐把差额变成消息上方空白。静态尺寸变化由 FOLLOW/READING 锚点处理，虚拟尺寸变化由 Virtualizer 处理，不做高度补偿动画。
驻留窗口止于断点轮次（其后还有未驻留的历史）时，该轮次不是会话的最后一轮：不得标记 `isLast`，也不得提供编辑/重跑。
optimistic user 被 ACK 替换为 canonical 消息时，Feed 与 Virtualizer 继续使用 `client_message_id` 作为节点身份，业务导航仍使用 canonical `round_id`，禁止重挂载同一轮或重播入场动画。

聊天排版偏好改变后，虚拟 Feed 在下一帧清除高度缓存，已挂载行继续用 ResizeObserver 测量。挂载时不清除：从静态 Feed 切换过来的首批测高是恢复阅读位置的依据。
