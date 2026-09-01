# Settings API

- `preferences-api.ts` 负责用户偏好；PATCH 把服务端单调 version 作为强 `If-Match` 前置条件，不把 version 写入正文或另造幂等 ID。`provider-api.ts` 负责 Provider 配置；向导和恢复流程可把 exact Provider `configuration_version` 作为强 `If-Match`，旧调用不带条件时保持兼容。
- `echo-api.ts` 负责当前用户共享的 Echo 开关，并用同一 Preferences revision 的强 `If-Match` 防止覆盖其他设置；单个 DM 的覆盖仍属于 conversation API。
- `runtime-api.ts` 只读取 `/settings/runtime/nxs/status`；宿主 workspace 属于部署配置，不能通过 HTTP 修改。
- `browser-api.ts` 只读取桌面浏览器扩展的连接状态，不承载浏览数据或动作。
