// INPUT: Connector 连接事实与当前命令忙碌状态。
// OUTPUT: 徽标和动作的本地化文案 key；状态优先级与命令目标保持独立。
// POS: Connector 目录纯展示模型，不读取语言或执行连接命令。

import type { TranslationKey } from "@/shared/i18n/messages";
import type { ConnectorInfo } from "@/types/capability/connector";

import { getConnectorState } from "../model/connector-state-model";

export interface ConnectorCardBadgeModel {
  labelKey: TranslationKey;
  tone?: "warning";
}

export type ConnectorCardTrailingModel =
  | { kind: "busy" | "coming-soon" }
  | {
      action: "connect" | "disconnect" | "select";
      ariaLabelKey: TranslationKey;
      icon: "connect" | "credential" | "disconnect" | "oauth-client";
      kind: "action";
    };

export interface ConnectorCardModel {
  badge: ConnectorCardBadgeModel | null;
  trailing: ConnectorCardTrailingModel;
}

export function buildConnectorCardModel(
  connector: ConnectorInfo,
  busy: boolean,
): ConnectorCardModel {
  const state = getConnectorState(connector);
  const needsOauthClient = state.oauthClientAction === "configure";
  const needsCredential = state.primaryAction === "configure-credential";
  const badge: ConnectorCardBadgeModel | null = state.status === "coming-soon"
    ? { labelKey: "capability.connector_card_coming_soon" }
    : needsOauthClient
      ? { labelKey: "capability.connector_card_needs_configuration", tone: "warning" }
      : null;

  let trailing: ConnectorCardTrailingModel;
  if (busy) {
    trailing = { kind: "busy" };
  } else if (state.status === "connected") {
    trailing = { action: "disconnect", ariaLabelKey: "capability.connector_action_disconnect_named", icon: "disconnect", kind: "action" };
  } else if (state.status === "coming-soon") {
    trailing = { kind: "coming-soon" };
  } else if (needsOauthClient || needsCredential) {
    trailing = { action: "select", ariaLabelKey: "capability.connector_action_configure_named",
      icon: needsOauthClient ? "oauth-client" : "credential", kind: "action" };
  } else {
    trailing = { action: state.primaryAction === "connect" ? "connect" : "select",
      ariaLabelKey: "capability.connector_action_connect_named", icon: "connect", kind: "action" };
  }
  return { badge, trailing };
}
