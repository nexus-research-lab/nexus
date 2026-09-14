// INPUT: 桌面运行环境、桥接可用性和持久化更新版本。
// OUTPUT: 串行轮询的可用版本提示，卸载后丢弃迟到结果。
// POS: 侧栏更新提示读取；不启动更新或重放写入。

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
    let refreshing = false;
    const refresh = async () => {
      if (!active || refreshing) return;
      refreshing = true;
      try {
        const result = await getDesktopPersistentState(UPDATE_STATE_KEY);
        if (active) {
          setVersion(result.value?.trim() || null);
        }
      } catch {
        // 更新提示是增强信息，不影响侧边栏主导航。
      } finally {
        refreshing = false;
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
