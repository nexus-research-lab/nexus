const SHOP_DOMAIN_PATTERN = /^[a-z0-9][a-z0-9-]*$/;

export const SHOP_DOMAIN_PROMPT_MESSAGE = "输入 myshopify.com 前面的店铺子域名。";
export const SHOP_DOMAIN_VALIDATION_MESSAGE = "请输入有效的 Shopify 店铺子域名";

export function normalizeShopDomain(value: string): string | null {
  const normalized = value
    .trim()
    .toLowerCase()
    .replace(/^https?:\/\//, "")
    .replace(/\/.*$/, "")
    .replace(/\.myshopify\.com$/, "");
  return SHOP_DOMAIN_PATTERN.test(normalized) ? normalized : null;
}
