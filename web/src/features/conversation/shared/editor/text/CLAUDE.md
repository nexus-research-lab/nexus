# Text File Editor

- Body 默认在失焦时退出编辑；需要显式保存确认的窄场景可关闭该行为，但保存状态仍由文件控制器维护。

- `text-file-editor.tsx` 只连接文件控制器、状态投影和窄视图，不拥有渲染策略。
- `text-file-editor-model.ts` 统一决定正文模式、工具栏状态和外部写入提示。
- `text-file-editor-recovery.ts` 只根据读取 revision、保存意图和 exact live 文件事实决定保存对账与实时更新；`text-file-editor-reliability.tsx` 使用统一资源状态展示 Problem / Impact / Recovery，不解释内部请求或 revision。
- Header 只组合文件元信息和命令；Body 只管理渲染器选择、尺寸观测和输入框焦点。
- Markdown 预览可以占满滚动视口，但正文行高与块间距只由共享 Markdown 配方决定；短内容的剩余高度必须留在文末，不参与段落分配。
- 文件编辑器与 Agent 资料编辑器必须将 exact `agentId` 透传到 Body/Content；Markdown 预览在消费侧绑定资源能力，不跟随全局当前 Agent 选择。
- 已识别的源码文本通过文件扩展名映射到共享 Prism 语义色板，只渲染内容本身；工作区 Header、复制动作和滚动仍归预览 chrome，未知纯文本继续使用无高亮 `<pre>`。
- API 保存反馈与外部实时写入状态分开呈现，不为同一写入事务维护重复状态。
- 文件必须先成功读取正文和 revision 才能编辑；保存始终携带该读取 revision。owner generation、Agent 或 path 变化立即重建状态，所有迟到读取、保存和对账结果都必须拒绝写回。
- dirty 草稿与编辑中的正文不接受实时外部内容覆盖。并发变化先保留草稿、读取最新内容，再由用户明确选择放弃草稿或以最新 revision 覆盖；任何选择都不得自动合并。
- 保存传输结果未知时锁定后续保存，只允许 GET 当前 exact Agent/path 文件进行内容与 revision 对账；页面不自动重放 PUT。明确 `not_applied`，或 exact GET 证明当前文件仍处于提交基线时，才允许用户明确再次保存；后者不得被描述成原请求从未短暂生效。401/403 清除旧正文和草稿。
- 新增文件类型时扩展渲染器或模式规则，不在入口组件追加条件分支。
