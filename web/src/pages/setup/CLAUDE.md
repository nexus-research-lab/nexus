# Setup Page

- `setup-page.tsx` 只承担首次 Deployment owner 初始化；成功后刷新统一 Auth 状态并进入 Launcher。
- 就绪页消费与 Login 共用的 Access 布局和宣传标题，并排字段选择 wide 表单列；表单与关闭状态说明使用公共 Panel，普通文本使用 Typography，不另写玻璃表面或字号配方。
- 表单使用 Panel `filled` 减弱装饰背景穿透，与 Login 共用同一填充配方。
- Setup capability 只保存在当前表单内，不写入 localStorage、URL 或日志。
- 结果不确定时先重新读取状态，不自动重复提交 owner 创建。
- `browser-tests/setup.spec.ts` 拦截所有初始化/认证 API，验证字段资格、键盘遍历、pending 阻塞、失败后只读对账、未开放状态与凭证不持久化；不得向真实 Control 创建测试账号。
