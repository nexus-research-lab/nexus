# Connector Device Flow

- `connector-device-auth-dialog.tsx` 只装配授权码、复制和跳转交互。
- `use-connector-device-auth.ts` 只负责轮询器随授权会话启停。
- `connector-device-auth-poller.ts` 独占定时器、退避和终态回调。

轮询器停止后不得再发出消息、错误或连接完成回调；服务端 `slow_down` 只调整下一轮间隔。
