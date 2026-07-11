# Launcher Console

- `launcher-console.tsx` 只组合 Tour、目录投影、控制器和 Hero。
- `launcher-console-helpers.ts` 保存最近会话、Mention 目标和装饰 Token 的纯投影。
- `launcher-console-types.ts` 定义 Console 与 Hero 的消费者接口。
- `use-launcher-console-controller.ts` 拥有查询互斥、服务端动作分发和会话导航。

服务端动作必须通过完整分发表执行。所有 Conversation 跳转共用同一导航入口，不在视图或动作分支中重复拼 URL。
