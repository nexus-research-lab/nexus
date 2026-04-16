"use client";

import { Fragment } from "react";
import { Link2 } from "lucide-react";

import type { ConnectorController } from "@/hooks/capability/use-connector-controller";

import { ConnectorCard } from "./connector-card";

interface ConnectorsGridProps {
  ctrl: ConnectorController;
}

/** 连接器卡片网格 */
export function ConnectorsGrid({ ctrl }: ConnectorsGridProps) {
  if (ctrl.loading) {
    return (
      <div className="flex min-h-40 items-center justify-center text-sm text-(--text-muted)">
        加载中…
      </div>
    );
  }

  if (ctrl.connectors.length === 0) {
    return (
      <div className="flex min-h-60 flex-col items-center justify-center gap-3 text-(--text-muted)">
        <div className="flex h-14 w-14 items-center justify-center rounded-full border border-(--divider-subtle-color) bg-(--surface-inset-background)">
          <Link2 className="h-6 w-6" />
        </div>
        <p className="text-sm">没有找到匹配的连接器</p>
      </div>
    );
  }

  const available = ctrl.connectors.filter((c) => c.status === "available");
  const coming_soon = ctrl.connectors.filter((c) => c.status === "coming_soon");

  return (
    <div className="flex flex-col gap-7">
      {available.length > 0 && (
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
      )}

      {coming_soon.length > 0 && (
        <Fragment>
          <div className="mb-3 flex items-center gap-2.5">
            <h2 className="text-[15px] font-bold tracking-[-0.02em] text-(--text-strong)">即将推出</h2>
            <span className="text-[11px] font-medium text-(--text-soft)">
              {coming_soon.length} 个
            </span>
          </div>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
            {coming_soon.map((connector) => (
              <ConnectorCard
                key={connector.connector_id}
                connector={connector}
                on_select={() => ctrl.open_detail(connector.connector_id)}
              />
            ))}
          </div>
        </Fragment>
      )}
    </div>
  );
}
