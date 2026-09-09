# CC Switch Provider 导入

- `provider-ccswitch-dialog.tsx` 只编排本机 CC Switch 预览、来源选择、同步和刷新确认；读取、同步与已提交后刷新失败必须保持独立结果语义。
- 初始预览读取使用共享 `md` muted Spinner，同步、路径检测和刷新使用共享 `sm` Spinner；按钮继承当前动作颜色，刷新图标使用 muted tone。
- 视图不得自行维护加载图标尺寸、颜色、旋转或 reduced-motion class，也不得因读取失败自动重放同步写入。

- 表单处理器与按钮同时拒绝加载中及 accepted/committed 再同步；同步和提交后刷新使用同一同步 ref 防重，未知结果仍保留既有同源幂等显式再同步。
