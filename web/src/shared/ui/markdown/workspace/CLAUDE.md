# Markdown 工作区资源

- `markdown-workspace-artifact-model.ts` 只负责文件索引、路径规则和 Markdown 附件分段，不读取 React 状态。
- `markdown-workspace-file-button.tsx` 将已解析路径传给消费侧绑定作用域的文件打开命令，不持有 Agent 身份。

消费侧必须让路径索引、图片 URL 和文件打开命令绑定同一资源作用域；共享层不读取业务 Store。歧义 basename 不得猜测。工作区相对图片路径不依赖文件树预加载，存在性与越界由文件接口校验。附件识别不得丢弃路径前后的正文，视图只消费已归一化结果。
