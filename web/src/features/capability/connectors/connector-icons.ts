/**
 * 连接器图标映射 —— 没有真实品牌图形时回退到这里的首字母/emoji。
 */

/** 首字母缩写 */
const CONNECTOR_ICON_LETTERS: Record<string, string> = {
  gmail: "G",
  "x-twitter": "𝕏",
  linkedin: "in",
  shopify: "S",
  instagram: "IG",
  github: "GH",
  "feishu-docx": "飞",
  amap: "高",
  didi: "滴",
  dingtalk: "钉",
  "tencent-docs": "腾",
  yuque: "语",
  "google-calendar": "GC",
  "google-drive": "GD",
  youtube: "YT",
  reddit: "R",
  tiktok: "TT",
  odoo: "O",
  square: "□",
  alibaba: "A",
  outlook: "O",
  airtable: "AT",
  meta: "M",
  ahrefs: "Ah",
  similarweb: "SW",
  dropbox: "DB",
  slack: "#",
  notion: "N",
  zapier: "Z",
  monday: "M",
  make: "Mk",
  linear: "L",
  atlassian: "A",
};

/** 获取图标首字母 */
export function get_connector_letter(icon: string, title: string): string {
  return CONNECTOR_ICON_LETTERS[icon] ?? title.charAt(0).toUpperCase();
}
