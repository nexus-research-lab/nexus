# Markdown 工作区资源

- `markdown-workspace-artifact-model.ts` 只负责文件索引、路径规则和 Markdown 附件分段，不读取 React 状态。
- `use-markdown-workspace-files.ts` 将当前 Agent 与文件 Store 适配为稳定的索引查询函数。
- `markdown-workspace-file-button.tsx` 把已解析路径适配为文件打开命令。

路径解析必须绑定当前 Agent；歧义 basename 不得猜测。附件识别不得丢弃路径前后的正文，视图只消费已归一化结果。
