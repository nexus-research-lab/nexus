# Settings API

- `preferences-api.ts` 负责用户偏好，`provider-api.ts` 负责 Provider 配置。
- `echo-api.ts` 只负责当前用户共享的 Echo 开关；单个 DM 的覆盖仍属于 conversation API。
- `runtime-api.ts` 只读取 `/settings/runtime/nxs/status`；宿主 workspace 属于部署配置，不能通过 HTTP 修改。
- `browser-api.ts` 只读取桌面浏览器扩展的连接状态，不承载浏览数据或动作。
