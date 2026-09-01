# Account API

- `auth-api.ts` 负责认证、个人资料、密码 exact request/终态回执核对与放弃、个人用量。
- `subscription-api.ts` 负责管理员订阅计划与用户订阅操作。
- `project-api.ts` 负责 owner-scoped 共享项目与成员 ACL 控制面。
- 账户接口不得混入 Provider 或普通应用设置协议。
