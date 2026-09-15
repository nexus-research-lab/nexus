// INPUT: 在线资源启用状态与可取消刷新动作。
// OUTPUT: 页面可见时的单飞刷新、恢复焦点/网络刷新和手动刷新入口。
// POS: Team 轻量元数据刷新；不重载消息历史。
import { useCallback, useEffect, useRef } from "react";

export function useTeamRefresh(scope: string | null, load: (signal: AbortSignal) => Promise<unknown>) {
  const latest = useRef(load);
  latest.current = load;
  const pending = useRef<AbortController | null>(null);
  const refresh = useCallback(() => {
    if (!scope || pending.current) return;
    const controller = new AbortController();
    pending.current = controller;
    void latest.current(controller.signal).finally(() => {
      if (pending.current === controller) pending.current = null;
    });
  }, [scope]);

  useEffect(() => {
    if (!scope) return;
    const refreshVisible = () => { if (document.visibilityState === "visible") refresh(); };
    // ponytail: M1 可见页面每 15 秒拉取元数据；多副本/大量在线用户时改为版本通知驱动。
    const timer = window.setInterval(refreshVisible, 15_000);
    document.addEventListener("visibilitychange", refreshVisible);
    window.addEventListener("focus", refreshVisible);
    window.addEventListener("online", refreshVisible);
    refreshVisible();
    return () => {
      window.clearInterval(timer);
      document.removeEventListener("visibilitychange", refreshVisible);
      window.removeEventListener("focus", refreshVisible);
      window.removeEventListener("online", refreshVisible);
      pending.current?.abort();
      pending.current = null;
    };
  }, [scope, refresh]);
  return refresh;
}
