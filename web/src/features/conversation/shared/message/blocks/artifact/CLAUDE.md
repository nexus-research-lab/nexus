# artifact/

L6 | 父级: web/src/features/conversation/shared/message/blocks

## 职责

结构化文件由 `workspace-file-artifacts.tsx` 在唯一入口解析归属：Artifact 自带的非空 workspace Agent 优先，其次为消息或节点传入的明确来源；没有来源时不读取全局当前 Agent。正文、收起态过程和运行历史都必须传入已知来源，集合不会因缺少预览 handler 而隐藏文件证据。Markdown 文件段继续由既有 workspace Markdown 适配器解析其上下文，再传给同一文件原语。

- `artifact-path-model.ts`: 统一路径归一化、文件名和父目录投影
- `workspace-artifact-action-model.ts`: 构造可执行的工作区外部动作
- `workspace-artifact-external-action.tsx`: 组合文字 Button 与公共反馈，通过 `hooks/agent/use-workspace-file-external-action.ts` 执行下载/桌面定位，不再自己调用 API 或吞掉失败
- `workspace-file-artifacts.tsx`: 适配结构化工作区文件产物
- `workspace-file-artifact-list-model.ts`: 按显式产物归属优先、来源工作区兜底及实际 path 去重；同目标保留最后记录、首次顺序，缺少来源不推断相等。显示名、工具 ID 不作为文件身份，历史证据不改写
- `workspace-file-artifact-utils.ts`: 从消息内容提取工作区文件产物
- `file/`: 文件块展示模型和视图
- `image/`: 图片来源解析、展示模型和视图

文件与图片不得分别调用下载 API 或解释桌面动作。工作区路径提取集中在本域模型，消息视图不重复解析 Markdown 文件引用。

图片详情和 WorkGraph 来源读取使用共享 `md` muted Spinner；Artifact 视图不得自行维护加载图标尺寸、颜色、旋转或 reduced-motion class。

WorkGraph 产物卡和来源对照只负责图数据与响应式编排：卡片、动作、文字、版本标记和窄屏 pane 切换分别复用 `UiPanel`、`UiButton`、Typography、`UiBadge` 与 `UiTabs`，不得再手写对照页签的按钮、阴影或选中态。
