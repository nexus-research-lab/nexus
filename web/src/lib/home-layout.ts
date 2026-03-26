/**
 * 首页工作台布局常量
 *
 * 把三栏宽度、间距和编辑器拖拽范围集中到这里，
 * 后面调布局时只改这一处，不用在多个组件里到处找类名。
 */

export const HOME_PAGE_PADDING_CLASS = "p-1 sm:p-4 xl:pt-4 xl:p-4 2xl:p-4";
export const HOME_WORKSPACE_SECTION_GAP_CLASS = "gap-2 xl:gap-3";
export const HOME_WORKSPACE_MAIN_GAP_CLASS = "gap-0 lg:gap-3 xl:gap-4";

export const HOME_WORKSPACE_SIDEBAR_WIDTH_CLASS =
  "w-[220px] shrink-0 lg:w-[240px] xl:w-[clamp(248px,18vw,300px)] 2xl:w-[clamp(264px,18vw,340px)]";

export const HOME_WORKSPACE_OBJECT_LIST_WIDTH_CLASS =
  "w-[220px] shrink-0 lg:w-[236px] xl:w-[clamp(244px,17vw,296px)] 2xl:w-[clamp(252px,17vw,320px)]";

export const HOME_AGENT_INSPECTOR_WIDTH_CLASS =
  "w-[208px] shrink-0 xl:w-[clamp(224px,16vw,296px)] 2xl:w-[clamp(248px,17vw,336px)]";

export const HOME_AGENT_INSPECTOR_WRAPPER_CLASS =
  "hidden min-[1280px]:flex min-[1280px]:min-h-0 min-[1280px]:shrink-0";

export const HOME_CHAT_PANEL_CLASS =
  "flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-transparent";

export const HOME_EDITOR_DEFAULT_WIDTH_PERCENT = 42;
export const HOME_EDITOR_MIN_WIDTH_PERCENT = 30;
export const HOME_EDITOR_MAX_WIDTH_PERCENT = 56;

export function clampHomeEditorWidthPercent(widthPercent: number): number {
  return Math.min(
    Math.max(widthPercent, HOME_EDITOR_MIN_WIDTH_PERCENT),
    HOME_EDITOR_MAX_WIDTH_PERCENT,
  );
}
