# personal/ - 个人设置

- `personal-settings-model.ts` 定义密码草稿、校验规则，并把可空资料与用量响应投影成非空展示模型，不访问 React 或 API。
- `use-personal-settings-controller.ts` 负责资料加载、头像保存、密码修改和反馈事务。
- `personal-profile-section.tsx`、`personal-password-section.tsx` 与 `personal-token-usage-section.tsx` 只消费窄 Props；`personal-avatar-picker.tsx` 复用统一头像弹层并保留保存和禁用反馈。
- 当前登录方式不支持修改密码时，只显示不可用原因，不渲染三项禁用输入与提交按钮；Token 总量、额度和输入/输出/缓存明细用于精确阅读，构成图用于快速比较比例，两者保留。
- 密码规则通过有序规则表表达；新增规则不得在视图或提交函数中复制条件分支。
- 资料与用量缺省值只在展示模型解释；Section 不得重复读取可空 API 字段。
- 头像与密码命令必须经过控制器互斥状态，视图不得直接调用 Auth API。
