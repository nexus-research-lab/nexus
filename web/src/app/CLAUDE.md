# app/

L2 | 父级: web/src

## 职责

- `app-providers.tsx` 只装配应用级 Provider。
- `auth/` 持有认证资源与用户作用域切换事务。
- `runtime-options-resource.ts` 拉取运行时配置并原子提交给 Config 快照。
- `router/` 与 `layout/` 分别拥有路由树和常驻应用壳层。
- 路由模板与资源身份编码统一消费 `shared/navigation/route-paths.ts`；App 不拥有供底层 Feature 反向依赖的路径常量。

应用级副作用不得下沉到 `shared/`；Bootstrap 只调用这里的完整启动命令，不解释资源协议。
