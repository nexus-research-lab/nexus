/**
 * 首页工作台布局常量
 *
 * 把三栏宽度、间距和侧栏拖拽范围集中到这里，
 * 后面调布局时只改这一处，不用在多个组件里到处找类名。
 */

/** 首页舞台使用贴近桌面窗口的薄边距，避免形成网页卡片感。 */
const HOME_STAGE_VERTICAL_PADDING_CLASS = "py-1.5";
export const HOME_PAGE_PADDING_CLASS = `pr-1.5 ${HOME_STAGE_VERTICAL_PADDING_CLASS}`;
export const HOME_SIDEBAR_PADDING_CLASS = `pl-1 pr-1.5 ${HOME_STAGE_VERTICAL_PADDING_CLASS}`;

export const HOME_SIDE_PANEL_DEFAULT_WIDTH_PERCENT = 56;
const HOME_SIDE_PANEL_MIN_WIDTH_PERCENT = 30;
const HOME_SIDE_PANEL_MAX_WIDTH_PERCENT = 56;

export function clampHomeSidePanelWidthPercent(widthPercent: number): number {
  return Math.min(
    Math.max(widthPercent, HOME_SIDE_PANEL_MIN_WIDTH_PERCENT),
    HOME_SIDE_PANEL_MAX_WIDTH_PERCENT,
  );
}
