/**
 * INPUT: 本地时钟与文档可见性。
 * OUTPUT: 当前应显示的品牌栏天空变体，在时段边界、窗口重新可见或获得焦点时更新。
 * POS: 天空变体的唯一时间订阅；规则来自 sidebar-sky-schedule，视图只消费结果。
 */
import { useEffect, useState } from "react";

import { msUntilNextSkyChange, resolveSidebarSkyVariant } from "./sidebar-sky-schedule";
import type { SidebarSkyVariant } from "./view/sidebar-night-sky";

export function useSidebarSkyVariant(): SidebarSkyVariant {
  const [variant, setVariant] = useState(() => resolveSidebarSkyVariant(new Date()));

  useEffect(() => {
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
  }, []);

  return variant;
}
