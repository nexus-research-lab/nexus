# file/

L7 | 父级: web/src/features/conversation/shared/message/blocks/artifact

## 职责

- `file-artifact-model.ts`: 只解析路径、Agent 作用域与交互资格，不返回 CSS 或密度
- `file-artifact-layout.ts`: 文件卡片的内容几何与排版；共享 content recipe 持有材质，外部动作仍由公共 Button 持有
- `file-artifact-block.tsx`: 展示文件信息并组合打开与外部动作

文件默认标签与打开提示消费共享国际化目录；显式空标签继续隐藏。预览打开与外部下载/显示是独立命令，预览缺少 handler 时只禁用该动作，不影响仍有 scope 的外部动作。显式 workspace Agent 始终优先于全局当前 Agent，切换当前 Agent 不能改写已有文件来源。

文件块不直接调用下载 API；浏览器下载和桌面 reveal 统一由 Artifact 根域动作执行。
