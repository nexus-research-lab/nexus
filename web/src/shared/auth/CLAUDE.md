# auth/

L3 | 父级: web/src/shared

## 职责

- `auth-context.ts` 只定义认证状态契约、Context 与消费 Hook。
- `auth-owner-identity.ts` 只把可信认证事实投影为既有 owner scope 字符串，并判断持久 marker 格式；不读写存储、不清空状态，不把客户端隔离标识变成服务端授权身份。
- `auth-owner-generation.ts` 只提供进程内 owner generation capture/fence 与清理完成后的订阅发布；推进、旧连接摘除和最终发布由 `app/auth/` 按此顺序独占，消费者只能判断回调是否仍属于当前 owner。

认证请求、登录事务和 Provider 生命周期属于 `app/auth/`，共享层不得反向持有应用装配。
