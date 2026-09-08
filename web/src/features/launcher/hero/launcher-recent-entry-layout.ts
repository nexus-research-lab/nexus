// INPUT: 最近入口的列表序号与条目总数。
// OUTPUT: Launcher Hero 专属的容器间距和渐入时序 recipe。
// POS: Launcher 最近入口布局所有者；控件尺寸、圆角、文字与状态样式仍由 shared/ui Button 管理。

const ENTRY_DELAY_START_MS = 580;
const ENTRY_DELAY_STEP_MS = 55;

export const LauncherRecentEntryLayout = {
  listClassName: "mx-auto mt-4 flex w-full max-w-[420px] flex-wrap items-center justify-center gap-1",
} as const;

export function getLauncherRecentEntryDelayMs(index: number): number {
  return ENTRY_DELAY_START_MS + index * ENTRY_DELAY_STEP_MS;
}

export function getLauncherHandoffDelayMs(entryCount: number): number {
  return getLauncherRecentEntryDelayMs(entryCount);
}
