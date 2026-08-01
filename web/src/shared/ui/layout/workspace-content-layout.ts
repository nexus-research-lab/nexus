/** 管理型页面铺满可用工作面，只由共享页面留白控制左右边界。 */
export const WORKSPACE_CONTENT_MAX_WIDTH_CLASS_NAME = "max-w-none";

/** 所有工作面正文与 Header 共用同一个响应式水平留白。 */
export const WORKSPACE_CONTENT_GUTTER_CLASS_NAME =
  "px-[var(--workspace-content-gutter)]";

/** 横向滚动区抵消页面留白后，仍用同一变量恢复内部内容基线。 */
export const WORKSPACE_CONTENT_BLEED_CLASS_NAME =
  "-mx-[var(--workspace-content-gutter)] px-[var(--workspace-content-gutter)]";

/** 目录条目在桌面工作面统一使用三列，窄窗按可读宽度逐级收拢。 */
export const WORKSPACE_CATALOG_GRID_CLASS_NAME =
  "grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3";

export const WORKSPACE_CONTENT_PAGE_CLASS_NAME = [
  "w-full",
  WORKSPACE_CONTENT_MAX_WIDTH_CLASS_NAME,
  WORKSPACE_CONTENT_GUTTER_CLASS_NAME,
  "py-5",
].join(" ");
