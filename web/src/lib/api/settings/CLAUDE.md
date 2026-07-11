# Settings API

- `preferences-api.ts` 负责用户偏好，`provider-api.ts` 负责 Provider 配置。
- `runtime-api.ts` 统一 `/settings/runtime` 设置和 nxs 状态，不拆成重复客户端。
- `system-api.ts` 只负责设置页消费的系统版本信息。
