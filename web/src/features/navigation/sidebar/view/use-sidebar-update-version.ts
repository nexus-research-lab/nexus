import { useEffect, useState } from "react";

import { isDesktopRuntime } from "@/config/desktop-runtime";
import {
  getDesktopPersistentState,
  isDesktopBridgeAvailable,
} from "@/lib/desktop-bridge";

const UPDATE_STATE_KEY = "desktop.update.available";
const UPDATE_STATE_POLL_INTERVAL_MS = 30_000;

export function useSidebarUpdateVersion(): string | null {
  const [version, setVersion] = useState<string | null>(null);

  useEffect(() => {
    if (!isDesktopRuntime() || !isDesktopBridgeAvailable()) {
      return;
    }

    let active = true;
    const refresh = async () => {
      try {
        const result = await getDesktopPersistentState(UPDATE_STATE_KEY);
        if (active) {
          setVersion(result.value?.trim() || null);
        }
      } catch {
        // 更新提示是增强信息，不影响侧边栏主导航。
      }
    };

    void refresh();
    const timer = window.setInterval(refresh, UPDATE_STATE_POLL_INTERVAL_MS);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, []);

  return version;
}
