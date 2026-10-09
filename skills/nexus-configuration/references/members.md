# 管理独立 Web 用户

仅使用宿主 `NEXUSCFG_COMMAND_PATH`。先 inspect members，以当前返回的操作定义为准。必须是有效平台 owner/admin 登录下的主智能体私聊，无需加入组织；普通平台成员、其他 Agent、Room、无登录后台任务和 Desktop Local 不具备此能力。

```bash
"$NEXUSCFG_COMMAND_PATH" --json inspect --domain members
"$NEXUSCFG_COMMAND_PATH" --json plan --domain members --operation create --input '{"username":"new-user","display_name":"新用户","role":"member","password":{"$secret":"member-password"}}'
```

新账号具有 Web 访问资格，初始不属于任何组织；不得自动加入创建者的组织。组织协作由账号本人另行创建组织或接受邀请。role 表示平台角色，不能用组织角色授予本能力。

创建时用户名至少 3 位，密码由管理员在确认卡片中输入；不要默认生成可预测密码，不索取、读取、打印或在 Bash 参数里写密码。主智能体不使用 `--secrets-stdin`。

取得 plan 后，以相同 input 和稳定 request ID 调用 apply，携带 `--expected-revision`。`members` 每次写入都会由宿主弹出真人确认卡片，`--confirm` 和对话中的文字同意不能替代此卡片。运行 Bash 时给等待确认留足时间（例如 timeout 600000）；超时或响应丢失后先 history/inspect 核对，不能换 request ID 盲目重试。

```bash
"$NEXUSCFG_COMMAND_PATH" --json apply --domain members --operation create --input '{"username":"new-user","display_name":"新用户","role":"member","password":{"$secret":"member-password"}}' --expected-revision '<plan.current_revision>' --request-id '<本次变更唯一且重试不变的ID>' --confirm
```

- `update`：target 必须是 inspect 返回的精确 `user_id`；input 可含 `display_name`、`role`、`status`（active/revoked）。省略字段保持不变；用户名和密码不通过该操作修改。
- `remove`：target 为精确 `user_id`，input 为 `{}`。撤销当前部署访问、使现有 Session 失效，保留账号、组织关系、Agent、工作区和历史。重新启用使用 update status=active。
- 平台 admin 仅能创建和管理普通 member；平台 owner 可管理 admin/member。不能通过此入口创建或修改 owner，也不能停用当前登录账号。
- 修改或移除前展示目标用户名和精确 ID；实际权限由 Control 在执行时重新核验，不能通过改 target、scope 或角色字段提权。
- 写后以 result/checks 及再次 inspect 的真实状态为准。手动入口为 Web 设置 → 运营 → 部署用户，是否可见取决于当前登录身份与部署实现。接口报错或页面不可见本身不代表版本过旧；按主 Skill 的“配置请求失败”区分已知事实和待排查原因，仅在确认部署缺少对应能力后建议升级 Nexus/Control。不能用组织邀请冒充独立 Web 用户创建，不回退旧命令。
- 若 inspect 返回 operation_forbidden，报告实际权限错误；不要据此推断平台 owner 必须加入组织，也不要绕过校验。独立 Web 用户管理要求 Nexus/Control 已部署对应实现，修改 Skill 本身不能改变服务端权限。
