# file/

L7 | 父级: web/src/features/conversation/shared/message/blocks/artifact

## 职责

- `file-artifact-model.ts`: 只解析路径、Agent 作用域与交互资格，不返回 CSS 或密度
- `file-artifact-layout.ts`: 文件卡片的内容几何与排版；共享 content recipe 持有材质，外部动作仍由公共 Button 持有
- `file-artifact-block.tsx`: 展示文件信息并组合打开与外部动作

文件默认标签与打开提示消费共享国际化目录；显式空标签继续隐藏。预览打开与外部下载/显示是独立命令，但路径与工作区资格共享同一纯投影；预览缺少 handler 时只禁用该动作。文件原语只接受上游解析的明确来源，不订阅全局当前 Agent。缺少路径或来源时保留名称/目录、禁用打开且不生成外部动作，说明通过 aria-describedby 关联到文件按钮；来源清空立即移除旧动作。文件名、路径和提示消费公共排版角色，阅读型复合按钮与图标框几何继续留在本领域。

文件块不直接调用下载 API；浏览器下载和桌面 reveal 统一由 Artifact 根域动作执行。
