import { Download } from "lucide-react";
import { useEffect, useState } from "react";

import { isDesktopRuntime } from "@/config/desktop-runtime";
import {
  getDesktopPersistentState,
  isDesktopBridgeAvailable,
} from "@/lib/desktop-bridge";

const UPDATE_STATE_KEY = "desktop.update.available";
const UPDATE_STATE_POLL_INTERVAL_MS = 30_000;

export function SidebarUpdateIndicator() {
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

  if (!version) {
    return null;
  }

  const label = `Nexus ${version} 可更新`;
  return (
    <a
      aria-label={label}
      className="relative flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-emerald-600 text-white transition-colors hover:bg-emerald-700"
      href="https://github.com/nexus-research-lab/nexus/releases/latest"
      rel="noreferrer"
      target="_blank"
      title={label}
    >
      <Download className="h-[17px] w-[17px]" />
      <span className="absolute -right-0.5 -top-0.5 h-2.5 w-2.5 rounded-full border-2 border-(--surface-sidebar-background) bg-amber-400" />
    </a>
  );
}
