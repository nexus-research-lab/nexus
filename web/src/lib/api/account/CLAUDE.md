# Account API

- `auth-api.ts` 负责认证、个人资料、密码 exact request/终态回执核对与放弃、个人用量；Server Web 登录直接走同源 Control，Desktop Local 继续使用本地端点。
- `control-api.ts` 负责首次 owner 设置与 Deployment member 管理，页面不得自行拼接 Control URL。
- `subscription-api.ts` 负责管理员订阅计划与用户订阅操作。
- `project-api.ts` 负责 owner-scoped 共享项目与成员 ACL 控制面。
- 账户接口不得混入 Provider 或普通应用设置协议。
