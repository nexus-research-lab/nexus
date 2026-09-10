# Shopify 店铺域名

- `shop-domain-model.ts` 只负责输入规范化与校验，不持有中文提示常量。
- `use-shop-domain-prompt.ts` 持有一次性输入请求及 Promise 结算边界。
- `shop-domain-prompt-dialog.tsx` 把 invalid 状态投影为当前语言的公共 Prompt 字段错误，用途说明保持可见；切换语言只更新文案，不重置草稿或结算请求。

命令层只能请求规范化后的店铺域名，不得创建临时 React Root 或持有弹窗组件生命周期。
