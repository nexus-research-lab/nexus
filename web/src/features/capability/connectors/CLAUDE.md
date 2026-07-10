# Connectors

- 根目录只保留目录入口和跨 catalog/detail 使用的图标。
- `catalog/` 负责搜索、分组和连接器列表。
- `detail/` 负责详情状态、主动作与能力展示。
- `auth/` 负责 OAuth、Device Flow、直接凭证和附加认证信息。
- `controller/` 负责列表、详情和命令状态的组合，不向视图暴露宽接口。
- 配置弹窗使用判别联合状态；异步命令共享唯一互斥入口。
