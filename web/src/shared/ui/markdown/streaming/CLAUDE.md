# Markdown 流式渲染

- `markdown-stream-blocks.ts`: 把不完整输入切成可稳定渲染的区块；增量解析器缓存前缀，保留最后两块重扫以支持列表续写，非前缀修正重新解析。
- `markdown-streaming.tsx`: 以一个持久 `MarkdownText` 身份组合静态区块与当前增量区块；本次挂载一旦进入流式态，终态继续按相同 `start_offset` 分块并切换为静态组件，避免代码、图片等后续块重挂载；所有流已由共享帧调度器合批，禁止再用每实例延迟值制造第二批异步高度提交，初次加载的历史消息直接走静态单块。
- `adaptive-stream-clock.ts`: 低速输入保持约 18–90 grapheme/s 与短暂抗抖缓冲；积压达到 180 字符时按 350ms 目标延迟提高片段大小，并允许合并已完成块。终态设定一次 500ms 排空截止点，实际完成仍受共享调度额度与浏览器帧可用性约束；信用不得超过已到达 backlog。
- `stream-frame-scheduler.ts`: 所有可见流共用最高 30Hz RAF，每帧只允许一条流提交并按身份轮转；字符需求由各流时钟决定，大块追赶每次最多 2048 grapheme，返回 0 的 buffering/节流流不占名额，空 backlog 立即退订。
- `stream-text-units.ts`: 追加时通过 `lib/text-graphemes.ts` 重分前一个尾单元与新 delta，保证跨 transport chunk 的 emoji ZWJ、肤色修饰符与组合附标仍作为一个展示单元；基础切分不在流式层重复实现。
- `use-smooth-streaming-markdown-content.ts`: 通过唯一 `useSmoothStreamingMarkdownState` 同时返回正文与真实渲染态，只对追加快照做单调追赶；增量字符原地追加到目标缓冲，大块 live 追加以有界字符预算或完成块合并追赶，长尾按长度将提交间隔延长到最多 96ms，终态继续排空已有 backlog；历史首挂、非前缀修正、页面重新可见或减少动态效果时立即对齐真实正文。

`use-smooth-streaming-markdown-content.test.tsx` 用真实时钟与调度器验证长尾降频、grapheme 边界、终态排空和快照修正。

相邻列表项属于同一个 Markdown 语义块，即使条目之间存在空行也不得拆开，否则有序列表会丢失连续计数；列表之外的已完成区块仍保持稳定身份。

流式层只处理时间和增量边界，不复制正文组件语义、工作区路径解析或 Mermaid 渲染状态。
live 流的最后一个 grapheme 在下一单元确认边界前保持在 target backlog 中，runtime 终态后必须立即纳入正常排空；若恢复态已经显示的尾单元被新 delta 扩展，只能原子重写相同长度的已显示前缀，新增后缀仍走原 backlog，禁止用全量同步制造高度跳变。
结构化消息入口必须在 live 文本为空时就挂载 MarkdownRenderer；首批正文因此是对空目标的追加，历史或恢复消息则用已有正文首挂并直接呈现。
Room 的阅读位置仍由会话滚动层维护；Markdown 平滑只改善阅读节奏，不得承担滚动补偿或掩盖布局抖动。
流式时钟不得根据一次短传输间隙切换到终态速度；只有 runtime 明确结束后才能排空保留缓冲。字符推进由共享帧调度器以最高 30Hz 唤醒，每帧最多一条流实际提交且聚合展示量不得随并行 Agent 数量线性放大；空 backlog 时可退订并在下一次追加时复用原 `AdaptiveStreamClock`。
正文流式层不得添加 opacity、transform、光标或光晕动画；视觉连续性只由字符时钟、稳定区块身份与会话层原有贴底逻辑提供。

显示公式的独占行分隔（含列表/引用前缀）与代码块一样拥有内部空行；不可在中途冻结为多个 Markdown 块。完成后继续复用相同起点和节点，正文语法与未闭合降级仍由 core 处理。

实现参考 Lobe UI streamdown 的积压完成块合并、长尾降频与前缀缓存思路（`lobehub/lobe-ui@2907fc1e`，`packages/streamdown/src/useSmoothStreamContent.ts` 与 `blockLexer.ts`）；不引入依赖，继续复用本仓 grapheme、Markdown 边界和显式 runtime 终态。
