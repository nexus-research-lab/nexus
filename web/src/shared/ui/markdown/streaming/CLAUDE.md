# Markdown 流式渲染

- `markdown-stream-blocks.ts`: 把不完整输入切成可稳定渲染的区块。
- `markdown-streaming.tsx`: 以一个持久 `MarkdownText` 身份组合静态区块与当前增量区块；本次挂载一旦进入流式态，终态继续按相同 `start_offset` 分块并切换为静态组件，避免代码、图片等后续块重挂载；所有流已由共享帧调度器合批，禁止再用每实例延迟值制造第二批异步高度提交，初次加载的历史消息直接走静态单块。
- `adaptive-stream-clock.ts`: 根据追加字符的到达速度与最长间隔先建立短暂抗抖缓冲，再以小数预算连续追赶；终态使用有上限的温和排空，避免传输间隙和完成瞬间触发突发刷新；公平池轮转等待必须完整累计，共享调度器未授予的当前 backlog 预算不得扣除，但累计信用不能超过已经到达的 backlog。
- `stream-frame-scheduler.ts`: 所有可见 Markdown 流共用一个最高 30Hz RAF 和固定的每帧 12 grapheme 总额度；每帧只允许一条流实际消费并按订阅者身份轮转，返回 0 的 buffering 流不占提交名额、调度器继续探测后续流，空 backlog 必须立即退订。
- `stream-text-units.ts`: 使用 `Intl.Segmenter` 以 grapheme 切分流式正文，追加时重分前一个尾单元与新 delta，保证跨 transport chunk 的 emoji ZWJ、肤色修饰符与组合附标仍作为一个展示单元；不支持时才退回 Unicode code point。
- `use-smooth-streaming-markdown-content.ts`: 只对追加快照做单调追赶；增量字符原地追加到目标缓冲，大块 live 追加也按有界追赶速度展示而不瞬间扩高，终态继续排空已有 backlog；历史首挂、非前缀修正、页面重新可见或减少动态效果时立即对齐真实正文。

相邻列表项属于同一个 Markdown 语义块，即使条目之间存在空行也不得拆开，否则有序列表会丢失连续计数；列表之外的已完成区块仍保持稳定身份。

流式层只处理时间和增量边界，不复制正文组件语义、工作区路径解析或 Mermaid 渲染状态。
live 流的最后一个 grapheme 在下一单元确认边界前保持在 target backlog 中，runtime 终态后必须立即纳入正常排空；若恢复态已经显示的尾单元被新 delta 扩展，只能原子重写相同长度的已显示前缀，新增后缀仍走原 backlog，禁止用全量同步制造高度跳变。
结构化消息入口必须在 live 文本为空时就挂载 MarkdownRenderer；首批正文因此是对空目标的追加，历史或恢复消息则用已有正文首挂并直接呈现。
Room 的阅读位置仍由会话滚动层维护；Markdown 平滑只改善阅读节奏，不得承担滚动补偿或掩盖布局抖动。
流式时钟不得根据一次短传输间隙切换到终态速度；只有 runtime 明确结束后才能排空保留缓冲。字符推进由共享帧调度器以最高 30Hz 唤醒，每帧最多一条流实际提交且聚合展示量不得随并行 Agent 数量线性放大；空 backlog 时可退订并在下一次追加时复用原 `AdaptiveStreamClock`。
正文流式层不得添加 opacity、transform、光标或光晕动画；视觉连续性只由字符时钟、稳定区块身份与会话层原有贴底逻辑提供。
