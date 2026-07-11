# Markdown 工作区资源

- `markdown-workspace-artifacts.ts`: 解析 Agent 作用域、文件路径和预览资源。
- `markdown-workspace-file-button.tsx`: 把已解析路径适配为文件打开命令。

路径解析必须绑定当前 Agent；视图只消费已归一化路径，不自行拼接预览 URL 或猜测文件来源。
