/**
 * INPUT: Connector 详情、连接状态、能力条目与文档地址。
 * OUTPUT: 本地化状态、能力和文档，优先展示连接准备步骤、默认折叠技术事实、连接后展示工具目录。
 * POS: Connector 详情正文纯视图。
 */
import type { ReactNode } from "react";
import { Check, ChevronRight, ExternalLink } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import { UiLinkButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import { UiListRow } from "@/shared/ui/list/list-row";
import { UiPanel } from "@/shared/ui/panel";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { ResourceFailure } from "@/lib/error-message";
import type {
  ConnectorDetail,
  ConnectorFeatureDetail,
  CustomMCPToolCatalog,
} from "@/types/capability/connector";

import { getConnectorCategoryLabel } from "../catalog/connectors-categories";
import { MCPToolsSection } from "../mcp/mcp-tools-section";
import type {
  ConnectorState,
  ConnectorStatusTone,
} from "../model/connector-state-model";
import { getConnectorAuthLabelKey } from "./connector-detail-model";

const STATUS_BADGE: Record<
  ConnectorStatusTone,
  (state: ConnectorState, t: I18nContextValue["t"]) => ReactNode
> = {
  connected: (_, t) => (
    <UiBadge tone="success">
      <Check className="h-3.5 w-3.5" />
      {t("capability.connector_status_connected")}
    </UiBadge>
  ),
  "coming-soon": (_, t) => <UiBadge>{t("capability.connector_card_coming_soon")}</UiBadge>,
  unconfigured: (state, t) => (
    <UiBadge tone="warning">
      {t(state.oauthClientAction ? "capability.connector_status_app_unconfigured" : "capability.connector_service_unconfigured")}
    </UiBadge>
  ),
  disconnected: (_, t) => <UiBadge>{t("capability.connector_status_disconnected")}</UiBadge>,
};

function ConnectorStatusBadges({
  detail,
  state,
}: {
  detail: ConnectorDetail;
  state: ConnectorState;
}) {
  const { t } = useI18n();
  return (
    <div className="flex flex-wrap gap-2">
      {STATUS_BADGE[state.status](state, t)}
      <UiBadge>{t(getConnectorAuthLabelKey(detail.auth_type))}</UiBadge>
      <UiBadge>{getConnectorCategoryLabel(detail.category, t)}</UiBadge>
      {detail.scopes.length > 0 ? (
        <UiBadge>{t("capability.connector_scopes_count", { count: detail.scopes.length })}</UiBadge>
      ) : null}
    </div>
  );
}

function ConnectorConfigurationError({
  error,
}: {
  error: string | null | undefined;
}) {
  const { t } = useI18n();
  if (!error) {
    return null;
  }
  return (
    <UiResourceState
      impact={t("capability.connector_configuration_unavailable_impact")}
      nextStep={t("capability.connector_configuration_unavailable_next_step")}
      size="sm"
      state="error"
      title={t("capability.connector_configuration_unavailable_title")}
    />
  );
}

function ConnectorFeatureList({
  features,
  onSelect,
}: {
  features: ConnectorFeatureDetail[];
  onSelect: (featureName: string) => void;
}) {
  const { t } = useI18n();
  if (features.length === 0) {
    return null;
  }
  return (
    <section>
      <h2 className={cn(
        "mb-2",
        getUiTypographyClassName({ role: "sectionTitle", tone: "strong" }),
      )}>
        {t("capability.connector_features_title")}
      </h2>
      <UiPanel
        className="divide-y divide-(--divider-subtle-color)"
        padding="none"
        radius="md"
        variant="card"
      >
        {features.map((feature) => (
          <UiListRow
            density="compact"
            description={feature.description}
            key={feature.name}
            onClick={() => onSelect(feature.name)}
            right={<ChevronRight className="h-4 w-4 shrink-0 text-(--icon-muted)" />}
            title={feature.name}
            variant="flush"
          />
        ))}
      </UiPanel>
    </section>
  );
}

function ConnectorDocsLink({ url }: { url: string | undefined }) {
  const { t } = useI18n();
  if (!url) {
    return null;
  }
  return (
    <UiLinkButton
      className="w-fit"
      href={url}
      rel="noopener noreferrer"
      size="sm"
      target="_blank"
      variant="text"
    >
      <ExternalLink className="h-3.5 w-3.5" />
      {t("capability.connector_docs")}
    </UiLinkButton>
  );
}

interface ConnectorMCPToolsState {
  catalog: CustomMCPToolCatalog | null;
  failure: ResourceFailure | null;
  loading: boolean;
  refresh: () => void;
  supported: boolean;
}

function RichMailConnectionSection({
  catalog,
  detail,
}: {
  catalog: CustomMCPToolCatalog | null;
  detail: ConnectorDetail;
}) {
  const { t } = useI18n();
  const connected = detail.connection_state === "connected";
  const serverName = catalog?.server_title || catalog?.server_name;
  return (
    <section className="space-y-3">
      {!connected ? (
        <UiPanel
          aria-label={t("capability.connector_richmail_prepare_title")}
          role="note"
          variant="card"
        >
          <h2 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>
            {t("capability.connector_richmail_prepare_title")}
          </h2>
          <ol className={cn(
            "mt-3 list-decimal space-y-3 pl-5",
            getUiTypographyClassName({ role: "supporting", tone: "default" }),
          )}>
            <li>{t("capability.connector_richmail_prepare_intro")}</li>
            <li>
              <p>{t("capability.connector_richmail_settings_path")}</p>
              <p className="mt-1">{t("capability.connector_richmail_enable_setting")}</p>
            </li>
            <li>{t("capability.connector_richmail_prepare_next")}</li>
          </ol>
        </UiPanel>
      ) : null}
      <UiDisclosure label={t("capability.connector_connection_info")}>
        <dl className="space-y-3">
          <ConnectorFact label={t("capability.connector_fact_endpoint")} value={detail.mcp_server_url || "http://127.0.0.1:3100/mcp"} />
          <ConnectorFact label={t("capability.connector_fact_transport")} value="Streamable HTTP" />
          <ConnectorFact label={t("capability.connector_fact_auth")} value={t("capability.connector_richmail_auth")} />
          {connected && serverName ? (
            <ConnectorFact
              label={t("capability.connector_fact_server")}
              value={catalog?.server_version ? `${serverName} · ${catalog.server_version}` : serverName}
            />
          ) : null}
          {connected && catalog?.protocol_version ? (
            <ConnectorFact label={t("capability.connector_fact_protocol")} value={catalog.protocol_version} />
          ) : null}
          {connected ? (
            <ConnectorFact label="Token" value={t("capability.connector_richmail_token_saved")} />
          ) : null}
        </dl>
      </UiDisclosure>
    </section>
  );
}

function ConnectorFact({ label, value }: { label: string; value: string }) {
  return (
    <div className={cn(
      "grid grid-cols-[88px_minmax(0,1fr)] gap-3",
      getUiTypographyClassName({ role: "supporting" }),
    )}>
      <dt className="ui-type-tone-muted ui-type-weight-medium">{label}</dt>
      <dd className="min-w-0 break-words [overflow-wrap:anywhere] ui-type-tone-default">{value}</dd>
    </div>
  );
}

export function ConnectorDetailContent({
  detail,
  features,
  mcpTools,
  onSelectFeature,
  state,
}: {
  detail: ConnectorDetail;
  features: ConnectorFeatureDetail[];
  mcpTools: ConnectorMCPToolsState;
  onSelectFeature: (featureName: string) => void;
  state: ConnectorState;
}) {
  const { t } = useI18n();
  return (
    <div className="mt-6 space-y-5">
      <ConnectorStatusBadges detail={detail} state={state} />
      <ConnectorConfigurationError error={state.configurationError} />
      {mcpTools.supported ? (
        <RichMailConnectionSection
          catalog={mcpTools.catalog}
          detail={detail}
        />
      ) : null}
      <ConnectorFeatureList features={features} onSelect={onSelectFeature} />
      {mcpTools.supported && detail.connection_state === "connected" ? (
        <MCPToolsSection
          available={detail.connection_state === "connected"}
          catalog={mcpTools.catalog}
          description={t("capability.connector_richmail_tools_description")}
          failure={mcpTools.failure}
          loading={mcpTools.loading}
          onRetry={mcpTools.refresh}
          unavailableMessage={t("capability.connector_richmail_tools_unavailable")}
        />
      ) : null}
      <ConnectorDocsLink url={detail.docs_url} />
    </div>
  );
}
