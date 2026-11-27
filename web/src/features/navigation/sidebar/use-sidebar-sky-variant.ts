/**
 * INPUT: 外观设置中的侧栏天空偏好、本地时钟与文档可见性。
 * OUTPUT: 当前应显示的品牌栏天空变体；关闭时为 null。跟随时间时在时段边界、窗口重新可见或获得焦点时更新。
 * POS: 天空变体的唯一时间订阅；规则来自 sidebar-sky-schedule，偏好来自 shared/theme，视图只消费结果。
 */
import { useEffect, useState } from "react";

import { useSidebarSkyPreference, type SidebarSkyVariant } from "@/shared/theme/sidebar-sky-preference";

import { msUntilNextSkyChange, resolveSidebarSkyVariant } from "./sidebar-sky-schedule";

export function useSidebarSkyVariant(): SidebarSkyVariant | null {
  const preference = useSidebarSkyPreference((state) => state.preference);
  const scheduled = useScheduledSkyVariant(preference === "auto");
  if (preference === "off") return null;
  return preference === "auto" ? scheduled : preference;
}

/** 只有跟随时间时才挂定时器和窗口事件；固定或关闭时停在最后一次结果，不做无用唤醒。 */
export function useScheduledSkyVariant(active = true): SidebarSkyVariant {
  const [variant, setVariant] = useState(() => resolveSidebarSkyVariant(new Date()));

  useEffect(() => {
    if (!active) return;
    let timer: ReturnType<typeof setTimeout> | undefined;
    // 每次刷新都重新排下一次边界，休眠唤醒或改系统时间后也能自我校正。
    const refresh = () => {
      const now = new Date();
      setVariant(resolveSidebarSkyVariant(now));
      clearTimeout(timer);
      timer = setTimeout(refresh, msUntilNextSkyChange(now));
    };
    const refreshWhenVisible = () => {
      if (document.visibilityState === "visible") refresh();
    };

    refresh();
    document.addEventListener("visibilitychange", refreshWhenVisible);
    window.addEventListener("focus", refresh);
    return () => {
      clearTimeout(timer);
      document.removeEventListener("visibilitychange", refreshWhenVisible);
      window.removeEventListener("focus", refresh);
    };
  }, [active]);

  return variant;
}
