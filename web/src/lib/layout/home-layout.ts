/**
 * 首页工作台布局常量
 *
 * 把三栏宽度、间距和编辑器拖拽范围集中到这里，
 * 后面调布局时只改这一处，不用在多个组件里到处找类名。
 */

/** 中文注释：首页舞台边距改成更贴近桌面窗口的薄边距，避免出现网页卡片感。 */
export const HOME_STAGE_VERTICAL_PADDING_CLASS = "py-1.5";
export const HOME_PAGE_PADDING_CLASS = `pr-1.5 ${HOME_STAGE_VERTICAL_PADDING_CLASS}`;
export const HOME_SIDEBAR_PADDING_CLASS = `pl-1 pr-1.5 ${HOME_STAGE_VERTICAL_PADDING_CLASS}`;

export const HOME_EDITOR_DEFAULT_WIDTH_PERCENT = 42;
export const HOME_EDITOR_MIN_WIDTH_PERCENT = 30;
export const HOME_EDITOR_MAX_WIDTH_PERCENT = 56;

export function clamp_home_editor_width_percent(widthPercent: number): number {
  return Math.min(
    Math.max(widthPercent, HOME_EDITOR_MIN_WIDTH_PERCENT),
    HOME_EDITOR_MAX_WIDTH_PERCENT,
  );
}
