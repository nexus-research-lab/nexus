/**
 * 连接器图标映射 —— 将 connector icon key 映射为颜色和首字母/emoji。
 * 前端不引入真实品牌 SVG，用颜色 + 字母代替。
 */

/** 连接器品牌配色 */
const CONNECTOR_ICON_COLORS: Record<string, { bg: string; text: string }> = {
  gmail: { bg: "bg-red-50", text: "text-red-600" },
  "x-twitter": { bg: "bg-slate-100", text: "text-slate-900" },
  linkedin: { bg: "bg-blue-50", text: "text-blue-700" },
  shopify: { bg: "bg-green-50", text: "text-green-700" },
  instagram: { bg: "bg-pink-50", text: "text-pink-600" },
  github: { bg: "bg-slate-100", text: "text-slate-800" },
  "google-calendar": { bg: "bg-blue-50", text: "text-blue-600" },
  "google-drive": { bg: "bg-amber-50", text: "text-amber-600" },
  youtube: { bg: "bg-red-50", text: "text-red-600" },
  reddit: { bg: "bg-orange-50", text: "text-orange-600" },
  tiktok: { bg: "bg-slate-100", text: "text-slate-900" },
  odoo: { bg: "bg-purple-50", text: "text-purple-700" },
  square: { bg: "bg-slate-100", text: "text-slate-800" },
  alibaba: { bg: "bg-orange-50", text: "text-orange-700" },
  outlook: { bg: "bg-blue-50", text: "text-blue-600" },
  airtable: { bg: "bg-teal-50", text: "text-teal-600" },
  meta: { bg: "bg-blue-50", text: "text-blue-600" },
  ahrefs: { bg: "bg-orange-50", text: "text-orange-600" },
  similarweb: { bg: "bg-blue-50", text: "text-blue-500" },
  dropbox: { bg: "bg-blue-50", text: "text-blue-600" },
  slack: { bg: "bg-purple-50", text: "text-purple-600" },
  notion: { bg: "bg-slate-100", text: "text-slate-900" },
  zapier: { bg: "bg-orange-50", text: "text-orange-600" },
  monday: { bg: "bg-violet-50", text: "text-violet-600" },
  make: { bg: "bg-violet-50", text: "text-violet-700" },
  linear: { bg: "bg-violet-50", text: "text-violet-600" },
  atlassian: { bg: "bg-blue-50", text: "text-blue-600" },
};

/** 首字母缩写 */
const CONNECTOR_ICON_LETTERS: Record<string, string> = {
  gmail: "G",
  "x-twitter": "𝕏",
  linkedin: "in",
  shopify: "S",
  instagram: "IG",
  github: "GH",
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

/** 获取图标配色，有默认值 */
export function get_connector_colors(icon: string): { bg: string; text: string } {
  return CONNECTOR_ICON_COLORS[icon] ?? { bg: "bg-slate-100", text: "text-slate-600" };
}

/** 获取图标首字母 */
export function get_connector_letter(icon: string, title: string): string {
  return CONNECTOR_ICON_LETTERS[icon] ?? title.charAt(0).toUpperCase();
}
