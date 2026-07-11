# Room Thread 实时数据

- `room-thread-live-store.ts` 保存当前会话发布的最小实时切片，不承担领域投影。
- `room-thread-panel-model.ts` 根据 Thread 目标纯派生面板模型。
- `use-room-thread-source.ts` 负责发布与清理会话切片，并在会话切换时关闭旧 Thread。
- `use-room-thread-panel.ts` 是桌面与移动端共用的消费入口。

实时源不保存可从上下文得到的会话标识；生产者不订阅自身发布的数据，避免反馈更新。
