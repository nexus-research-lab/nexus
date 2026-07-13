# config/

L2 | 父级: web/src

## 职责

- `runtime-endpoints.ts` 只解析浏览器与桌面宿主的 API/WebSocket 地址。
- `runtime-options.ts` 保存当前用户作用域的 Agent 与偏好快照，并提供原子应用入口；值清洗复用 `lib/settings/`，不在配置层复制规则。
- `conversation-policy.ts` 保存构建期固定的会话策略常量。
- `desktop-runtime/` 独占桌面宿主协议与生命周期。

配置层不得请求网络或依赖 Feature；运行时配置拉取由 `app/runtime-options-resource.ts` 编排。
