/**
 * INPUT: 用户设备的本地时间。
 * OUTPUT: 当前时段对应的品牌栏天空变体，以及距离下一次切换的毫秒数。
 * POS: 天空变体随时间切换的唯一规则表；纯函数，不读取定时器或 DOM。
 */
import type { SidebarSkyVariant } from "@/shared/theme/sidebar-sky-preference";

// 按起始小时升序排列；跨零点的夜间段由最后一项延续到次日首项之前。
const SKY_SCHEDULE: ReadonlyArray<readonly [startHour: number, variant: SidebarSkyVariant]> = [
  [5, "dawn"],
  [9, "day"],
  [18, "night"],
  [22, "meteor"],
];

export function resolveSidebarSkyVariant(now: Date): SidebarSkyVariant {
  const hour = now.getHours();
  let variant = SKY_SCHEDULE[SKY_SCHEDULE.length - 1][1];
  for (const [startHour, candidate] of SKY_SCHEDULE) {
    if (hour >= startHour) variant = candidate;
  }
  return variant;
}

export function msUntilNextSkyChange(now: Date): number {
  const hour = now.getHours();
  const nextStart = SKY_SCHEDULE.find(([startHour]) => startHour > hour)?.[0] ?? SKY_SCHEDULE[0][0] + 24;
  const next = new Date(now);
  next.setHours(nextStart, 0, 0, 0);
  return Math.max(1_000, next.getTime() - now.getTime());
}
