# Shopify 店铺域名

- `shop-domain-model.ts` 只负责输入规范化与校验。
- `use-shop-domain-prompt.ts` 持有一次性输入请求及 Promise 结算边界。
- `shop-domain-prompt-dialog.tsx` 只把判别状态投影为共享 Prompt Dialog。

命令层只能请求规范化后的店铺域名，不得创建临时 React Root 或持有弹窗组件生命周期。
