# Channel Login

- `use-channel-login-controller.ts` 独占扫码会话、串行轮询和验证码提交生命周期。
- `channel-login-model.ts` 负责登录状态到标签与视觉语义的投影。
- `channel-login-panel.tsx` 只展示二维码、验证码和运行输出。
- 登录写命令复用连接控制器提供的命令入口，不建立第二把互斥锁。
- 启动登录只允许发生在保存配置事务内部，避免出现无配置的孤立登录会话。
