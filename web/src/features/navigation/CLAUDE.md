# 应用导航域

本目录负责应用壳层导航编排；业务目录内容由各 Feature 提供，通用视觉原语保留在 `shared/ui/`。

- `sidebar/` 管理宽侧栏的路由、Store、通知、调整尺寸和折叠/展开装配。
- `direct-room/` 负责 Agent 私聊目标解析、失效 Agent 恢复和 Conversation 路由生成。
- 导航 Feature 可以组合业务入口，但视图不得直接请求 API 或修改跨域状态。
