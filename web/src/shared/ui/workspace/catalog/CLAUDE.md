# Workspace Catalog UI

本目录只保存跨领域复用的 Workspace 目录视觉原语，不解释 Agent、Room 或任务领域状态。

## 边界

- `workspace-catalog-card.tsx` 只负责静态卡片框架与语义化 Ghost 动作，不把 Article 伪装成按钮。
- `workspace-catalog-content.tsx` 只负责标题、正文、标签和内容区布局。
- `workspace-catalog-actions.tsx` 只负责目录动作按钮外观。
- `workspace-icon-frame.tsx` 只负责图标容器的尺寸、形状和色调。
- 消费者按职责直接导入具体模块；不得恢复混合导出的聚合入口。
- 领域判断、权限、状态文案和命令互斥留在所属 Feature。
