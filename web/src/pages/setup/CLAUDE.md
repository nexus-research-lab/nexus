# Setup Page

- `setup-page.tsx` 只承担首次 Deployment owner 初始化；成功后刷新统一 Auth 状态并进入 Launcher。
- Setup capability 只保存在当前表单内，不写入 localStorage、URL 或日志。
- 结果不确定时先重新读取状态，不自动重复提交 owner 创建。
