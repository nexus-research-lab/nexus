# 工作区文件预览

- `workspace-file-preview-panel.tsx` 只编排空状态与已选文件状态；路径是打开态的唯一来源。
- `workspace-file-preview-router.tsx` 用完整渲染表分派文件类型，不在面板堆叠类型分支。
- `workspace-file-preview-types.ts` 只描述工作区内嵌预览的真实契约，不保留无消费者的独立宽度或拖拽模式。
- `workspace-file-preview-chrome.tsx` 统一文件标题、元信息、下载与聚焦操作，不读取文件内容。
- `workspace-file-preview-kind.ts` 只负责扩展名分类；具体加载、解析和渲染归各文件类型子目录。

通用布局能力不得反向放入本目录。新增文件类型时先扩展分类和路由描述表，再由对应子域拥有实现。
