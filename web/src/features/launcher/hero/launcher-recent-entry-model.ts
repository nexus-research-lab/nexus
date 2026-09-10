// INPUT: 最近 DM/Room 条目与当前语言的类型名称。
// OUTPUT: 截断标签、完整可访问名称和 Tooltip 需求等纯业务展示数据。
// POS: Launcher 最近入口数据投影；不返回 DOM、样式、颜色、尺寸、阴影或动画参数。

import type { RecentLauncherEntry } from "../console/launcher-console-types";
import {
  isLauncherChipTruncated,
  truncateLauncherChipLabel,
} from "../console/launcher-console-helpers";

export interface LauncherRecentEntryModel {
  ariaLabel: string;
  chipLabel: string;
  entry: RecentLauncherEntry;
  tooltipLabel: string | null;
}

export type LauncherRecentEntryTypeLabels = Record<
  RecentLauncherEntry["type"],
  string
>;

export function buildLauncherRecentEntryModel(
  entry: RecentLauncherEntry,
  typeLabels: LauncherRecentEntryTypeLabels,
): LauncherRecentEntryModel {
  const labelPrefix = entry.type === "room" ? "#" : "";
  const fullLabel = `${labelPrefix}${entry.label}`;
  return {
    ariaLabel: `${typeLabels[entry.type]} ${entry.label}`,
    chipLabel: `${labelPrefix}${truncateLauncherChipLabel(entry.label)}`,
    entry,
    tooltipLabel: isLauncherChipTruncated(entry.label) ? fullLabel : null,
  };
}
