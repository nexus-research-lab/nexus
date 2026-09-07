# 工作区文件预览

- `workspace-file-preview-panel.tsx` 只编排空状态与已选文件状态；路径是打开态的唯一来源。
- 空预览标题和说明必须跟随当前界面语言，不能把英文标题或中文说明固定在共享面板中。
- `workspace-file-preview-router.tsx` 用完整渲染表分派文件类型，不在面板堆叠类型分支；文本类型直接共用稳定 TextPreviewRenderer，Office 继续懒加载并复用同一加载外壳，不按渲染时机创建新的 renderer 身份。
- `workspace-file-preview-types.ts` 只描述工作区内嵌预览的真实契约，不保留无消费者的独立宽度或拖拽模式。
- `workspace-file-preview-chrome.tsx` 统一文件位置、状态、下载与聚焦操作，不读取文件内容。它只把 Agent 显示名、相对父目录和文件名投影给全站 `UiBreadcrumb`；不得自行绘制箭头、拼接斜杠或定义层级字级。位置、必要状态与纯图标操作共用单行 chrome，文件名保持紧凑中等字重，不重复显示文件类型“预览”标签。所有标题栏图标动作固定复用 `UiIconButton size="sm" variant="ghost"`，不得再维护私有按钮 class；保存动作保持稳定槽位，内容边界只保留一条底部结构线。
- 文件外部操作、聚焦、编辑、预览、保存和同步状态的文案必须由当前界面语言生成；纯展示模型接收翻译函数，不读取 React 上下文或保存固定中文。
- 外部文件动作的失败事实由 chrome 中的动作组件持有；只允许当前 owner 代次、Agent、路径、文件名及最近一次显式操作提交反馈。切换或卸载使旧反馈失效，文案在当前 render 翻译；这不取消已发出的下载/宿主操作，也不自动重放。对应行为见 `workspace-file-actions.test.tsx`。
- `workspace-file-preview-panel.test.tsx` 固定预览 scope 边界：相同文件的专注、位置文案和语言变化保留 renderer；Agent/path 变化或关闭会卸载它，旧本地草稿不能进入另一文件。
- `workspace-file-preview-kind.ts` 只负责扩展名分类；具体加载、解析和渲染归各文件类型子目录。
- `media/CLAUDE.md` 描述图片/PDF 的共用原生生命周期与 HTML 的流式文档提交；PDF 不从 iframe load/error 推断成功失败，显式重新加载继续复用统一文件工具栏。
- `workspace-file-preview-loading.tsx` 独占预览正文的共享 ResourceState/Spinner 组合；PDF、图片、Office、表格和文本入口只投影已有加载状态。Header 保留计数与独立写入同步事实，正文状态不再重复到 Header；布局与字号规则见根 `design.md`。
- `use-office-preview-scope.ts` 独占 DOCX/XLSX/PPTX 的 owner 代次、Agent/path、本地切换代次与显式重试状态；返回曾打开的文件也不得接受原来的回调。owner 尚未发布时旧任务即失去提交资格。各格式继续负责 AbortController、解析、DOM/对象 URL 释放；这些本地代次不构成服务端资源身份或缓存协议。
- 大型文本在整文件读取被服务端拒绝后只用 HTTP Range 分段只读展示，不在 WebView 中拼接；PDF 交给浏览器 Range，图片与 Office 等不可安全分段的预览由服务端限制载荷。

通用布局能力不得反向放入本目录。新增文件类型时先扩展分类和路由描述表，再由对应子域拥有实现。
