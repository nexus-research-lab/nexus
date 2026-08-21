# 工具执行块

- `tool-block-model.ts` 只组装唯一执行阶段、权限详情和结果摘要，并向 Composer 紧凑确认面暴露同一必要参数与可读权限建议投影；工具标题只能调用 `tool-activity.ts` 的统一 i18n resolver，状态和权限字段也由 i18n 上下文一次投影，模型不得保存固定语言或回退显示 raw MCP 名，也不得为 `status` 再维护运行中、待确认等镜像布尔值。消息级执行已停止且缺少 provider `tool_result` 时必须显式投影 `stopped`，不得继续显示 running。
- `use-tool-block-controller.ts` 管理展开、复制和权限选择；外层 ToolUseSummary 过程组点开后可用 `defaultExpanded` 让首次挂载直接展示完整输入与结果，主组件只编排头部、进度、结果与权限详情。
- `header/` 由纯投影统一解释可点击性、详情回退和动作能力，具体视图只渲染窄状态。
- `subagent-task-tool-entry.tsx` 只把 Agent/Task 启动投影成固定宽度的单行任务入口：头部用精确 `tool_use_id` 生成子智能体曲线头像，中间任务名必须截断，尾部只保留执行状态；点击把同一工具身份和调用者交给上游子智能体面板。不得重复通用工具卡的 Agent 标题、状态徽标、live tool 文案或蓝色进度面。待权限确认时继续复用完整 ToolBlock，不能绕开 Composer 的唯一决策面。
- `generative-ui-block.tsx`、`generative-ui-height-model.ts` 与 `generative-ui-document.ts` 把内建 `show_widget` 投影为对话内稳定 iframe；宿主只保留标题和中性表面，不显示工具图标、外边框或标题分隔线，iframe 注入 Nexus 三套主题及其图表色令牌。流式阶段增量解析并保持已完成 DOM 可交互，脚本开始生成后显示等待态；iframe 报告高度在 live 阶段只可单调增长，终态缩减需等待同一 Feed 的异步布局结算窗口再一次提交，不能让连续 resize 往返改变 Room 几何。终态在同一 iframe 内重建 DOM 并按顺序执行脚本，只有收到 ready 后才结束等待，语法、加载或运行错误必须回报宿主并原位展示。iframe 只允许脚本且不得获得宿主同源权限，网络与 CDN 资源保持开放。
- 权限字段标签由模型一次投影，头部色彩和详情滚动布局由具体视图持有，视图不得读取模型内部目录。Composer 持有操作时，DM/Room 正文里的待确认工具只使用中性静态证据样式，不得以 warning 色或脉冲争夺确认按钮的视觉焦点。
- 文件工具在折叠态只展示路径末级文件名，展开态与 Composer 权限确认保留完整路径，紧凑阅读不能牺牲授权判断所需上下文。
- 工具输入按 `unknown` 收窄，不把协议边界的动态值扩散为 `any`。
