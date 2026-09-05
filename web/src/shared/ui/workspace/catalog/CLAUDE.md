# Workspace Catalog UI

本目录只保存跨领域复用的 Workspace 目录视觉原语，不解释 Agent、Room 或任务领域状态。

## 边界

- `workspace-catalog-card.tsx` 负责卡片框架、可选 `primaryAction` 覆盖命中区与语义化 Ghost 动作。主动作是 Article 内独立的共享 Button，与内容中的聊天、删除等次动作互为兄弟，不把 Article 伪装成按钮。消费者不得重复覆盖按钮、焦点圈或整卡 hover 配方。
- `workspace-catalog-card.css` 仅拥有带主动作时的局部堆叠和命中路由：静态内容透传给底部主按钮，原生次动作继续独立命中，不要求业务复制定位和 pointer-events 配方。
- `workspace-catalog-content.tsx` 只负责标题、正文、标签和内容区布局。
- `workspace-catalog-actions.tsx` 只负责目录动作按钮外观。
- `workspace-icon-frame.tsx` 只负责图标容器的尺寸、形状和色调；默认图标基座使用共享暖色控制面和轻阴影，不得回退为高亮纯白圆块。
- 消费者按职责直接导入具体模块；不得恢复混合导出的聚合入口。
- 领域判断、权限、状态文案和命令互斥留在所属 Feature。
