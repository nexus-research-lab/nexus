// INPUT: 用户输入的 Shopify 店铺子域名或既有兼容 URL 格式。
// OUTPUT: 规范化子域名或无效结果。
// POS: 店铺域名纯模型；本地化提示由视图和请求控制器持有。

const SHOP_DOMAIN_PATTERN = /^[a-z0-9][a-z0-9-]*$/;

export function normalizeShopDomain(value: string): string | null {
  const normalized = value
    .trim()
    .toLowerCase()
    .replace(/^https?:\/\//, "")
    .replace(/\/.*$/, "")
    .replace(/\.myshopify\.com$/, "");
  return SHOP_DOMAIN_PATTERN.test(normalized) ? normalized : null;
}
