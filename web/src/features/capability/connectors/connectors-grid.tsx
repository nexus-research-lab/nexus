"use client";

import { Link2 } from "lucide-react";

import type { ConnectorController } from "@/hooks/use-connector-controller";

import { ConnectorCard } from "./connector-card";

interface ConnectorsGridProps {
  ctrl: ConnectorController;
}

/** 连接器卡片网格 */
export function ConnectorsGrid({ ctrl }: ConnectorsGridProps) {
  if (ctrl.loading) {
    return (
      <div className="flex min-h-40 items-center justify-center text-sm text-slate-400">
        加载中…
      </div>
    );
  }

  if (ctrl.connectors.length === 0) {
    return (
      <div className="flex min-h-60 flex-col items-center justify-center gap-3 text-slate-400">
        <div className="flex h-14 w-14 items-center justify-center rounded-full border border-white/40">
          <Link2 className="h-6 w-6" />
        </div>
        <p className="text-sm">没有找到匹配的连接器</p>
      </div>
    );
  }

  // 分组：先显示可用的（按连接状态排序），再显示即将推出的
  const available = ctrl.connectors.filter((c) => c.status === "available");
  const coming_soon = ctrl.connectors.filter((c) => c.status === "coming_soon");

  return (
    <div className="space-y-7">
      {/* 可用连接器 */}
      {available.length > 0 && (
        <section>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
            {available.map((connector) => (
              <ConnectorCard
                key={connector.connector_id}
                busy={ctrl.busy_id === connector.connector_id}
                connector={connector}
                on_connect={() => void ctrl.handle_connect(connector.connector_id)}
                on_disconnect={() => void ctrl.handle_disconnect(connector.connector_id)}
                on_select={() => ctrl.open_detail(connector.connector_id)}
              />
            ))}
          </div>
        </section>
      )}

      {/* 即将推出 */}
      {coming_soon.length > 0 && (
        <section>
          <h3 className="mb-3 flex items-center gap-2 text-[13px] font-semibold text-slate-500">
            即将推出
            <span className="rounded-full bg-white/50 px-2 py-0.5 text-[11px] font-medium text-slate-400">
              {coming_soon.length}
            </span>
          </h3>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
            {coming_soon.map((connector) => (
              <ConnectorCard
                key={connector.connector_id}
                connector={connector}
                on_select={() => ctrl.open_detail(connector.connector_id)}
              />
            ))}
          </div>
        </section>
      )}
    </div>
  );
}
