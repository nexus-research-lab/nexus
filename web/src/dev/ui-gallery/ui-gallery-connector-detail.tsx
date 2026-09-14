// INPUT: 本地 RichMail 连接状态、长名称、端点和能力权限样例。
// OUTPUT: 实际 Connector 详情、准备说明和能力弹窗的窄工作面预览。
// POS: 开发夹具；不调用连接命令、认证或 MCP 工具目录请求。

import { useState } from "react";

import { ConnectorDetailContent } from "@/features/capability/connectors/detail/connector-detail-content";
import { ConnectorDetailHeader } from "@/features/capability/connectors/detail/connector-detail-header";
import { ConnectorFeatureDialog } from "@/features/capability/connectors/detail/connector-feature-dialog";
import { getConnectorState } from "@/features/capability/connectors/model/connector-state-model";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { ConnectorDetail, ConnectorFeatureDetail } from "@/types/capability/connector";

import { galleryText } from "./ui-gallery-copy";

const FEATURE: ConnectorFeatureDetail = {
  name: "Mailbox capability", description: "Read and search the mailbox approved by the user.",
  items: ["Find messages without modifying their content.", "Read the selected mailbox."],
  scopes: [`mail:${"account/".repeat(28)}read`],
};
const DETAIL: ConnectorDetail = {
  connector_id: "richmail", name: "richmail", icon: "richmail", kind: "connector",
  title: "RichMail-MultiAccountMailboxAndCalendarConnectorWithAnExtendedName",
  description: "A connector with a long description for reviewing how details adapt to a narrow workspace pane.",
  category: "productivity", auth_type: "local_pairing", connection_state: "disconnected",
  is_configured: true, status: "available", scopes: FEATURE.scopes!,
  mcp_server_url: `https://example.test/${"mail/".repeat(36)}mcp`,
  features: [FEATURE.name], feature_details: [FEATURE],
};
const noAction = () => undefined;

export function ConnectorDetailGallery() {
  const { locale } = useI18n();
  const [busy, setBusy] = useState(false);
  const [connected, setConnected] = useState(false);
  const [feature, setFeature] = useState<ConnectorFeatureDetail | null>(null);
  const detail: ConnectorDetail = { ...DETAIL, connection_state: connected ? "connected" : "disconnected" };
  const state = getConnectorState(detail);
  return <section className="min-w-0 space-y-4 xl:col-span-2" data-gallery-connector-detail>
    <h2 className={getUiTypographyClassName({ role: "pageTitle", tone: "strong" })}>
      {galleryText(locale, "连接器详情与能力预览", "Connector details and capabilities")}
    </h2>
    <UiButton aria-pressed={busy} data-gallery-connector-busy onClick={() => setBusy((value) => !value)} variant="outline">
      {galleryText(locale, "切换忙碌状态", "Toggle busy state")}
    </UiButton>
    <div className="min-w-0 max-w-full" data-gallery-connector-detail-body>
      <ConnectorDetailHeader busy={busy} detail={detail} state={state}
        onConfigureCredential={noAction} onConfigureOauthClient={noAction} onReplaceOauthClient={noAction}
        onConnect={() => setConnected(true)} onDisconnect={() => setConnected(false)} />
      <ConnectorDetailContent detail={detail} features={[FEATURE]} state={state}
        mcpTools={{ catalog: null, failure: null, loading: false, refresh: noAction, supported: true }}
        onSelectFeature={() => setFeature(FEATURE)} />
    </div>
    <ConnectorFeatureDialog connectorTitle={detail.title} feature={feature} onClose={() => setFeature(null)} />
  </section>;
}
