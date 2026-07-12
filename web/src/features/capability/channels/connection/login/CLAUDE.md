# Channel Login

- `use-channel-login-controller.ts` 独占扫码会话、串行轮询和验证码提交生命周期。
- `channel-login-model.ts` 将登录协议快照统一投影为 idle/session 状态，并集中标签、视觉语义、身份、输出与验证码提示。
- `channel-login-panel.tsx` 按 Header、二维码、验证码和会话输出拆分窄视图，不解释原始状态字段。
- `login-qr-code.tsx` 独占二维码动态生成、失效响应拒绝和图片展示。
- 登录写命令复用连接控制器提供的命令入口，不建立第二把互斥锁。
- 启动登录只允许发生在保存配置事务内部，避免出现无配置的孤立登录会话。
