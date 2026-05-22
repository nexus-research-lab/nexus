export const CONNECTOR_CATEGORY_OPTIONS = [
  { key: "all", label: "全部" },
  { key: "productivity", label: "效率工具" },
  { key: "social", label: "社交媒体" },
  { key: "ecommerce", label: "电商平台" },
  { key: "development", label: "开发工具" },
  { key: "business", label: "企业管理" },
  { key: "marketing", label: "营销分析" },
  { key: "automation", label: "自动化" },
];

export function get_connector_category_label(category: string): string {
  return CONNECTOR_CATEGORY_OPTIONS.find((item) => item.key === category)?.label ?? category;
}
