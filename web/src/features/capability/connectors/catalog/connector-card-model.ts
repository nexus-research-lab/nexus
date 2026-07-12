import type { ConnectorInfo } from "@/types/capability/connector";

import { getConnectorState } from "../model/connector-state-model";

export interface ConnectorCardBadgeModel {
  label: string;
  tone?: "warning";
}

export type ConnectorCardTrailingModel =
  | { kind: "busy" | "coming-soon" | "connected" }
  | {
      action: "connect" | "select";
      ariaLabel: string;
      icon: "connect" | "credential" | "oauth-client";
      kind: "action";
    };

export interface ConnectorCardModel {
  badge: ConnectorCardBadgeModel | null;
  trailing: ConnectorCardTrailingModel;
}

interface CardRule<Value> {
  matches: boolean;
  value: Value;
}

export function buildConnectorCardModel(
  connector: ConnectorInfo,
  busy: boolean,
): ConnectorCardModel {
  const state = getConnectorState(connector);
  const needsOauthClient = state.oauthClientAction === "configure";
  const needsCredential = state.primaryAction === "configure-credential";
  return {
    badge: firstCardValue<ConnectorCardBadgeModel | null>([
      {
        matches: state.status === "coming-soon",
        value: { label: "即将推出" },
      },
      {
        matches: needsOauthClient,
        value: { label: "待配置", tone: "warning" },
      },
      { matches: true, value: null },
    ]),
    trailing: firstCardValue<ConnectorCardTrailingModel>([
      { matches: busy, value: { kind: "busy" } },
      { matches: state.status === "connected", value: { kind: "connected" } },
      {
        matches: state.status === "coming-soon",
        value: { kind: "coming-soon" },
      },
      {
        matches: needsOauthClient,
        value: {
          action: "select",
          ariaLabel: `配置 ${connector.title}`,
          icon: "oauth-client",
          kind: "action",
        },
      },
      {
        matches: needsCredential,
        value: {
          action: "select",
          ariaLabel: `配置 ${connector.title}`,
          icon: "credential",
          kind: "action",
        },
      },
      {
        matches: true,
        value: {
          action: state.primaryAction === "connect" ? "connect" : "select",
          ariaLabel: `连接 ${connector.title}`,
          icon: "connect",
          kind: "action",
        },
      },
    ]),
  };
}

function firstCardValue<Value>(rules: CardRule<Value>[]): Value {
  const rule = rules.find((candidate) => candidate.matches);
  if (!rule) throw new Error("连接器卡片规则缺少兜底项");
  return rule.value;
}
