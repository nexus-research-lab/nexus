/**
 * INPUT: 应用窄窗交接阈值与首页舞台/侧栏几何约束。
 * OUTPUT: JS 媒体查询、对应 Tailwind 可见性类及首页布局常量。
 * POS: App 与 Conversation 共用的响应式边界真相；Feature 不复制 559px。
 */

/**
 * 首页舞台顶部与桌面窗口齐平，消除旧标题栏留下的悬空间隙；
 * 左右与底部仍保留薄边距，避免内容整体贴满窗口。
 */
const HOME_STAGE_BOTTOM_PADDING_CLASS = "pb-1.5";
export const HOME_PAGE_PADDING_CLASS = `pr-1.5 ${HOME_STAGE_BOTTOM_PADDING_CLASS}`;
export const HOME_SIDEBAR_PADDING_CLASS =
  `pl-[var(--sidebar-shell-leading-padding,4px)] pr-1.5 ${HOME_STAGE_BOTTOM_PADDING_CLASS}`;

/** 仅在真正窄屏切换单窗专注模式；中等宽度继续使用可渐进压缩的桌面工具栏。 */
export const APP_NARROW_VIEWPORT_MEDIA_QUERY = "(max-width: 559px)";
export const APP_NARROW_VIEWPORT_HIDDEN_CLASS_NAME = "max-[559px]:hidden";

export const HOME_SIDE_PANEL_DEFAULT_WIDTH_PERCENT = 56;
const HOME_SIDE_PANEL_MIN_WIDTH_PERCENT = 30;
const HOME_SIDE_PANEL_MAX_WIDTH_PERCENT = 56;

export function clampHomeSidePanelWidthPercent(widthPercent: number): number {
  return Math.min(
    Math.max(widthPercent, HOME_SIDE_PANEL_MIN_WIDTH_PERCENT),
    HOME_SIDE_PANEL_MAX_WIDTH_PERCENT,
  );
}
