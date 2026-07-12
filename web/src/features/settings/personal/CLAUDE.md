# personal/ - 个人设置

- `personal-settings-model.ts` 定义密码草稿、校验规则，并把可空资料与用量响应投影成非空展示模型，不访问 React 或 API。
- `use-personal-settings-controller.ts` 负责资料加载、头像保存、密码修改和反馈事务。
- `personal-profile-section.tsx`、`personal-password-section.tsx` 与 `personal-token-usage-section.tsx` 只消费窄 Props。
- 密码规则通过有序规则表表达；新增规则不得在视图或提交函数中复制条件分支。
- 资料与用量缺省值只在展示模型解释；Section 不得重复读取可空 API 字段。
- 头像与密码命令必须经过控制器互斥状态，视图不得直接调用 Auth API。
