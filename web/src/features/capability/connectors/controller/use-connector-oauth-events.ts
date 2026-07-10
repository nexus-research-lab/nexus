import { useEffect } from "react";

import { subscribeConnectorOauthEvent } from "../auth/connector-oauth-events";
import type { ReportConnectorFeedback } from "./connector-controller-types";

interface UseConnectorOauthEventsOptions {
  connectorId?: string;
  openDetail: (connectorId: string) => Promise<void>;
  refreshCatalog: () => Promise<void>;
  reportFeedback: ReportConnectorFeedback;
}

export function useConnectorOauthEvents({
  connectorId,
  openDetail,
  refreshCatalog,
  reportFeedback,
}: UseConnectorOauthEventsOptions) {
  useEffect(() => subscribeConnectorOauthEvent((event) => {
    reportFeedback({
      tone: event.type === "connector-oauth:success" ? "success" : "error",
      title: event.type === "connector-oauth:success"
        ? "操作完成"
        : "操作失败",
      message: event.message || (
        event.type === "connector-oauth:success"
          ? "连接成功"
          : "OAuth 连接失败"
      ),
    });
    void Promise.all([
      refreshCatalog(),
      connectorId ? openDetail(connectorId) : Promise.resolve(),
    ]);
  }), [connectorId, openDetail, refreshCatalog, reportFeedback]);
}
