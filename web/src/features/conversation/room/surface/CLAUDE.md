# Room Surface

- 根目录保留桌面/移动端入口和可独立展示的业务 Surface。
- `room-surface-model.ts` 放置桌面与移动端共享的纯派生，不读取 UI 状态。
- 桌面分栏、右侧面板与 Thread 编排统一位于 `layout/`。
- 会话历史排序、能力投影、标题编辑和条目视图统一位于 `history/`。
