# Launcher Console

- `launcher-console.tsx` 只组合 Tour、目录投影、控制器和 Hero。
- `launcher-console-helpers.ts` 保存最近会话、Mention 目标和装饰 Token 的纯投影。
- `launcher-console-types.ts` 定义 Console 与 Hero 的消费者接口。
- `use-launcher-console-controller.ts` 拥有查询互斥、服务端动作分发和会话导航。

服务端动作必须通过完整分发表执行。所有 Conversation 跳转共用同一导航入口，不在视图或动作分支中重复拼 URL。

顶部 Header 的横向灯组保持源动画 `356:41` 的比例：Web 使用 `139×16px` 保留识别度，桌面宿主收敛为 `104×12px`，macOS 位于系统窗口控制右侧并与其中心对齐；品牌图标与字标独占下一行，不与窗口控制共享基线。Striper 字款同样按环境分离：Web 保持原始紧凑字距，桌面宿主使用 `0.1em` 加宽字距，两者都使用字体文件真实的 Regular 字重，不合成伪粗体。
