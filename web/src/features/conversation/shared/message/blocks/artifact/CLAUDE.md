# artifact/

L6 | 父级: web/src/features/conversation/shared/message/blocks

## 职责

- `artifact-path-model.ts`: 统一路径归一化、文件名和父目录投影
- `workspace-artifact-action-model.ts`: 构造可执行的工作区外部动作
- `workspace-artifact-external-action.tsx`: 统一浏览器下载和桌面 reveal
- `workspace-file-artifacts.tsx`: 适配结构化工作区文件产物
- `workspace-file-artifact-utils.ts`: 从消息内容提取工作区文件产物
- `file/`: 文件块展示模型和视图
- `image/`: 图片来源解析、展示模型和视图

文件与图片不得分别调用下载 API 或解释桌面动作。工作区路径提取集中在本域模型，消息视图不重复解析 Markdown 文件引用。
