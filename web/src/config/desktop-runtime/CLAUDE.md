# 桌面运行时配置

## 协议边界

- `index.ts` 是消费者唯一入口，只重导出稳定的具体函数和类型。
- `runtime-config.ts` 解析宿主注入配置并设置文档级平台标记。
- `session-auth.ts` 管理 HTTP header、WebSocket subprotocol 和 token 失效恢复；恢复优先使用稳定 FailureCore code，并保留旧安全文案兼容。
- `oauth.ts` 管理连接器 OAuth 回调与桌面回跳地址。
- `lifecycle.ts` 管理 WebView ready、fatal、health 消息和诊断快照。
- `desktop-location.ts` 统一 URL 与路径归一化，避免协议各自解释地址。

## 约定

- 新宿主字段先加入配置字段映射，不在消费者直接读取原始全局对象。
- 窗口 chrome 只通过文档级平台标记、`--desktop-window-controls-inset` 与 `--desktop-window-close-button-center-x/y` 暴露横向控制区和红灯双轴中心；Y 只用于推导共享顶栏高度，不得作为垂直 inset 推低整棵应用树。
- `app-region` 只服务 macOS 统一标题栏；Windows 的 WebView 始终是客户区，配置层不得启动区域投影或复制拖动手势。
- macOS 与 Windows 的首屏窗口尺寸、最小尺寸和屏幕留白保持数值对齐，但原生 chrome 所有权不得对齐：macOS 使用 full-size Web 内容与透明标题栏，Windows 使用独立 34 DIP 原生栏并让 WebView 从下一行开始；该边界由前端基础契约同时检查两个宿主源码。
- 鉴权、OAuth 和生命周期协议不得互相复制 URL 判断。
- 原生消息 payload 的 snake_case 转换只允许出现在 `lifecycle.ts` 边界。
- 桌面恢复逻辑必须有可证明的重试上限；存储不可用时保持当前页面。
