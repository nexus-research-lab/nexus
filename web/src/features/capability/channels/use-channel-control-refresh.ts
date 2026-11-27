// INPUT: 频道或配对目录的快照读取函数（silent 表示后台读取，不改写用户反馈）。
// OUTPUT: 前台 refresh，以及由 `resource_changed{channel_control}` 驱动的后台重读。
// POS: Channel 控制状态的实时失效适配层；频道目录与配对目录共用同一失效规则。
import { useCallback, useRef } from "react";

import { isResourceChange } from "@/lib/websocket/protocol/resource-change";
import { useAppEventRefresh } from "@/lib/websocket/use-app-event-refresh";
import type { EventMessage } from "@/types/generated/protocol";

const isChannelControlChange = (event: EventMessage) => isResourceChange(event, "channel_control");

export function useChannelControlRefresh(
  read: (silent: boolean) => Promise<boolean>,
): (silent?: boolean) => Promise<boolean> {
  const inFlightRef = useRef<Promise<boolean> | null>(null);
  const refresh = useCallback((silent = false): Promise<boolean> => {
    const run = read(silent);
    inFlightRef.current = run;
    const settle = () => {
      if (inFlightRef.current === run) inFlightRef.current = null;
    };
    void run.then(settle, settle);
    return run;
  }, [read]);

  // 命令后的前台对账优先：后台读取等它收口再发，避免把前台读取判为过期而误报刷新失败。
  const refreshInBackground = useCallback(async (): Promise<boolean> => {
    while (inFlightRef.current) {
      await inFlightRef.current.catch(() => false);
    }
    return refresh(true);
  }, [refresh]);

  // 适配器连接就绪度没有推送，回到窗口时补读一次。
  useAppEventRefresh({
    onEvent: isChannelControlChange,
    refresh: refreshInBackground,
    revalidateOnFocus: true,
  });
  return refresh;
}
