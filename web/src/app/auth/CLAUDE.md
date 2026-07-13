# auth/

L3 | 父级: web/src/app

## 职责

- `auth-provider.tsx` 编排登录状态、登录/登出命令与用户作用域切换后的运行时配置刷新。
- 无状态 Context 与消费 Hook 留在 `shared/auth/auth-context.ts`，供 Feature 直接读取。

认证副作用属于应用装配层；`shared/` 不得持有运行时配置请求或用户切换事务。
