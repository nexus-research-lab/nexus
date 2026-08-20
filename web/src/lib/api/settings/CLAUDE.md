# Settings API

- `preferences-api.ts` 负责用户偏好，`provider-api.ts` 负责 Provider 配置。
- `runtime-api.ts` 只读取 `/settings/runtime/nxs/status`；宿主 workspace 属于部署配置，不能通过 HTTP 修改。
- `webbridge-api.ts` 只读取桌面浏览器扩展的连接状态，不承载浏览数据或动作。
