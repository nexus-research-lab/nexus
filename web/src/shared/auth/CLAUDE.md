# auth/

L3 | 父级: web/src/shared

## 职责

- `auth-context.ts` 只定义认证状态契约、Context 与消费 Hook。

认证请求、登录事务和 Provider 生命周期属于 `app/auth/`，共享层不得反向持有应用装配。
