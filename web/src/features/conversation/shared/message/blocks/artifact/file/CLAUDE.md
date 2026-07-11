# file/

L7 | 父级: web/src/features/conversation/shared/message/blocks/artifact

## 职责

- `file-artifact-model.ts`: 解析路径、Agent 作用域、交互资格和密度样式
- `file-artifact-block.tsx`: 展示文件信息并组合打开与外部动作

文件块不直接调用下载 API；浏览器下载和桌面 reveal 统一由 Artifact 根域动作执行。
