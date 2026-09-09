# personal/ - 个人设置

- `personal-settings-model.ts` 定义密码草稿、校验规则，并把可空资料与用量响应投影成非空展示模型，不访问 React 或 API。
- `use-personal-settings-controller.ts` 负责资料加载、头像保存、密码修改和反馈事务。
- `personal-profile-section.tsx`、`personal-password-section.tsx` 与 `personal-token-usage-section.tsx` 只消费窄 Props；`personal-avatar-picker.tsx` 复用统一头像弹层并保留保存和禁用反馈。
- Personal 的身份、套餐、元数据、区块标题、Token 数字与校验文案只选择 App Typography、Badge、Shape 和 Settings Card 语义；不得在各 Section 重新拼字号、字重、行高、tracking 或任意圆角。
- 当前登录方式不支持修改密码时，只显示不可用原因，不渲染三项禁用输入与提交按钮；Token 总量和输入/输出/缓存压缩为一行摘要，仅在配置额度时显示额度，精确数值及更新信息通过共享 Tooltip 查看。
- 密码规则通过有序规则表表达；新增规则不得在视图或提交函数中复制条件分支。
- 资料与用量缺省值只在展示模型解释；Section 不得重复读取可空 API 字段。
- 头像与密码命令必须经过控制器互斥状态，视图不得直接调用 Auth API。
- 密码修改每次生成 exact `request_id`；服务端以 `committed|not_applied` 终态回执在同一身份上提交或阻止迟到写入。前端只持久 user-scoped request 指针，不保存密码草稿；unknown 不创建 fresh request，有原草稿时显式续行同一 request，草稿丢失时必须先由服务端原子放弃再解锁。
- 资料初始加载使用共享 `lg` Spinner，头像与密码保存使用 `xs/sm`；个人设置不维护独立的尺寸、颜色或 reduced-motion class。

个人页内容限制为 max-w-4xl 并居中；身份、用量、安全按顺序排列。头像与名字居中，相同用户名不重复显示，角色和登录方式放入身份徽标的 Tooltip；密码表单使用原生 details 按需展开。接口提供累计值与最近 365 个 UTC 自然日聚合；personal-usage-history.tsx 使用真实每日数据渲染独立的年度 Token 活动热力图区块，下方单独保留近 30 天堆叠柱状图与明细表切换，不推算缺失历史。

`personal-sections.test.tsx` 验证用量唯一明细和密码折叠/提交行为。

热力图始终生成最近 365 天的 UTC 日历，固定 7 行紧凑格子，不按接口返回条数拉伸；缺失日期显示暂无记录。每日、每周和累计均基于窗口内已记录用量，使用共享 Tooltip 显示数值，与下方柱状图的切换状态独立。

用量摘要、坐标轴、热力图悬浮和柱状图详情按语言压缩为 K/M/B 或千/万/百万/亿，明细表保留精确数值。密码区使用右侧箭头展开，宽屏标签与输入框左右对齐，窄屏上下排列。
