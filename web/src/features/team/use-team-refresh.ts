// INPUT: 在线资源启用状态与可取消刷新动作。
// OUTPUT: 目录 WS 失效驱动的单飞刷新，以及焦点/网络恢复与手动重试。
// POS: Team 元数据恢复入口；复用 owner-scoped 共享连接，不设固定轮询。
import { useCallback, useEffect, useRef } from "react";
import { useWebSocket } from "@/lib/websocket/use-socket";
import { buildTeamStreamUrl } from "@/lib/api/conversation/team-api";

export function useTeamRefresh(scope: string | null, load: (signal: AbortSignal) => Promise<unknown>, watchDirectory = true) {
  const latest = useRef(load);
  latest.current = load;
  const pending = useRef<AbortController | null>(null);
  const invalidated = useRef(false);
  const refresh = useCallback(() => {
    if (!scope) return;
    if (pending.current) { invalidated.current = true; return; }
    invalidated.current = false;
    const controller = new AbortController();
    pending.current = controller;
    void latest.current(controller.signal).finally(() => {
      if (pending.current !== controller) return;
      pending.current = null;
      // 读取期间收到的 WS 事件不能丢失；合并成一次后续对账，不并发读取。
      if (!controller.signal.aborted && invalidated.current) refresh();
    });
  }, [scope]);

  // 多个资源沿共享 socket registry 复用同一条 owner-scoped 连接，重连初始提示同样补读。
  useWebSocket({
    autoConnect: Boolean(scope) && watchDirectory,
    reconnect: true,
    url: scope && watchDirectory ? buildTeamStreamUrl("directory", "directory") : "",
    onMessage: (message) => {
      const event = message as {type?: string; stream_id?: string};
      if (event?.type === "stream.updated" && event.stream_id === "directory") refresh();
    },
  });

  useEffect(() => {
    if (!scope) return;
    const refreshVisible = () => { if (document.visibilityState === "visible") refresh(); };
    document.addEventListener("visibilitychange", refreshVisible);
    window.addEventListener("focus", refreshVisible);
    window.addEventListener("online", refreshVisible);
    refreshVisible();
    return () => {
      document.removeEventListener("visibilitychange", refreshVisible);
      window.removeEventListener("focus", refreshVisible);
      window.removeEventListener("online", refreshVisible);
      pending.current?.abort();
      pending.current = null;
      invalidated.current = false;
    };
  }, [scope, refresh]);
  return refresh;
}
