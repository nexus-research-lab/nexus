# 桌面运行时配置

## 协议边界

- `index.ts` 是消费者唯一入口，只重导出稳定的具体函数和类型。
- `runtime-config.ts` 解析宿主注入配置并设置文档级平台标记。
- `session-auth.ts` 管理 HTTP header、WebSocket subprotocol 和 token 失效恢复。
- `oauth.ts` 管理连接器 OAuth 回调与桌面回跳地址。
- `lifecycle.ts` 管理 WebView ready、fatal、health 消息和诊断快照。
- `desktop-location.ts` 统一 URL 与路径归一化，避免协议各自解释地址。

## 约定

- 新宿主字段先加入配置字段映射，不在消费者直接读取原始全局对象。
- 鉴权、OAuth 和生命周期协议不得互相复制 URL 判断。
- 原生消息 payload 的 snake_case 转换只允许出现在 `lifecycle.ts` 边界。
- 桌面恢复逻辑必须有可证明的重试上限；存储不可用时保持当前页面。
