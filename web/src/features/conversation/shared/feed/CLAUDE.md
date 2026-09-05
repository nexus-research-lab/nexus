# feed/

L4 | 父级: web/src/features/conversation/shared

## 职责

- `conversation-feed.tsx`: DM 静态/虚拟消息流入口
- `conversation-virtual-feed.tsx`: 虚拟列表装配
- `conversation-round.tsx`: 静态与虚拟分支共用轮次渲染
- `conversation-feed-model.ts`: refs、renderer、source 与轮次状态投影
- `conversation-feed-tail.tsx`: 静态与虚拟 Feed 共用、不改变增长节奏的唯一底部锚点
- `conversation-virtual-canvas.tsx`: 将完整虚拟高度贴住 Feed 底部，并把 live 高度负债留在消息画布上方
- `use-conversation-virtual-metrics.ts`: viewport 宽度和导航偏移测量
- `use-conversation-virtual-scroll-policy.ts`: 稳定轮次 key、静态 Feed 切换时的初始偏移快照与动态高度锚点策略
- `use-conversation-virtualization-policy.ts`: 会话内锁存 static/virtual renderer，并只在 live epoch 结束后的稳定窗口跨越阈值
- `use-conversation-round-navigation.ts`: 静态与虚拟列表共用轮次导航

高度估算必须同时依赖轮次身份、消息分组和容器宽度；不得用 ref 或仅用数组长度规避 Hook 依赖。
DM 与 Room、静态与虚拟 Feed 必须复用 `conversation-panel-styles.ts` 的内容轨道宽度，保证超宽屏扩展和虚拟高度估算使用同一真实容器。
虚拟尺寸变化服从共享 scroll owner：bottom 或轮次导航事务活跃时 Virtualizer 不写 `scrollTop`；FOLLOW 中正向 measured delta 由 Virtualizer 保持底部，负向 delta 由 Feed 级 live 高度保护吸收；READING 空闲时完整位于视口上方的轮次继续补偿，用户直接滚动的短暂 epoch 内不得反向写回，横跨视口的流式长回复不得推走阅读位置。局部展开/收起不得在 Virtualizer 之外再补写同一尺寸 delta。
静态 Feed 只在 live epoch 结束后的稳定窗口跨过虚拟化阈值；Virtualizer 必须继承同一滚动容器的既有 `scrollTop`，不得在并行输出中途替换 renderer 或以默认零偏移覆盖用户正在阅读的位置。
`leadingContent` 只承载不属于 transcript 的本地视图前导，并必须进入静态 Feed 根节点，与消息轮次共同接受 live 高度保护和调用方指定的起始/实时对齐；只有前导且没有消息轮次时静态 Feed 必须至少填满 viewport，让欢迎页可按真实可见区居中，嵌入式顶部接待说明仍保持自身起始对齐；存在该前导时不得切换为未计入其高度的虚拟画布，禁止把前导放在 Feed 外制造两段内容栈。
活跃轮次的虚拟估高不能消费尚未 reveal 的完整 backend target；先使用保守基线，再由真实 Markdown/工具 DOM 的正向 measured delta 驱动增长。终态历史仍按完整内容估高。
索引中存在但正文尚未驻留的轮次必须在虚拟 Feed 中保留估算高度，不能让空 wrapper 的真实测量把它压成几像素；否则旧历史会从可滚动空间消失，可见窗口 loader 也失去稳定触发面。
轮间距必须放在每个可测量 round wrapper 内，静态父容器不得使用 `gap`/`space-y`；否则跨过虚拟化阈值时累计间距会消失并造成整屏位移。
虚拟 Feed 根节点必须声明 `data-conversation-virtual-feed="true"`，避免共享滚动层重复执行可见轮次补偿或 bottom 写入。
最后一个 root 不得预建 viewport 高度的空白 runway。新消息从真实内容底部进入；FOLLOW 把并行 Agent 视为同一条 live timeline，父 Feed 聚合尺寸变化后始终贴住真实底部，让旧内容连续上推。live epoch 可在父 Feed 暂存由估高校正和 shell 收口产生的高度负债，但静态内容栈与完整虚拟画布必须贴住 Feed 底部，让负债只占据消息上方而不能在真实 tail 后形成空洞；所有来源终态并经过短暂异步布局结算后只允许一次原子释放，不得做连续高度动画。不得给单个 Agent 预留 spacer 或在 terminal 后永久保留空白。
optimistic user 被 ACK 替换为 canonical 消息时，Feed 与 Virtualizer 继续使用 `client_message_id` 作为节点身份，业务导航仍使用 canonical `round_id`，禁止重挂载同一轮或重播入场动画。
