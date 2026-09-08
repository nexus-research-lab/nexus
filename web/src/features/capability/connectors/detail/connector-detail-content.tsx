/**
 * INPUT: Connector 详情、连接状态、能力条目与文档地址。
 * OUTPUT: 本地化状态、能力和文档，具可换行事实与准备步骤的公共连接说明面板。
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
  const serverName = catalog?.server_title
    || catalog?.server_name
    || t("capability.connector_richmail_metadata_pending");
  return (
    <section className="border-y border-(--divider-subtle-color) py-5">
      <h2 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>
        {t("capability.connector_connection_info")}
      </h2>
      <dl className="mt-3 grid grid-cols-[repeat(auto-fit,minmax(min(100%,280px),1fr))] gap-x-8 gap-y-3">
        <ConnectorFact label={t("capability.connector_fact_endpoint")} value={detail.mcp_server_url || "http://127.0.0.1:3100/mcp"} />
        <ConnectorFact label={t("capability.connector_fact_transport")} value="Streamable HTTP" />
        <ConnectorFact label={t("capability.connector_fact_auth")} value={t("capability.connector_richmail_auth")} />
        <ConnectorFact
          label={t("capability.connector_fact_server")}
          value={catalog?.server_version
            ? `${serverName} · ${catalog.server_version}`
            : serverName}
        />
        <ConnectorFact
          label={t("capability.connector_fact_protocol")}
          value={catalog?.protocol_version || t("capability.connector_richmail_metadata_pending")}
        />
        <ConnectorFact
          label="Token"
          value={t(connected ? "capability.connector_richmail_token_saved" : "capability.connector_richmail_token_pending")}
        />
      </dl>
      {!connected ? (
        <UiPanel
          aria-label={t("capability.connector_richmail_prepare_title")}
          className="mt-5"
          role="note"
          variant="filled"
        >
          <h3 className={getUiTypographyClassName({ role: "control", tone: "strong", weight: "semibold" })}>
            {t("capability.connector_richmail_prepare_title")}
          </h3>
          <p className={cn("mt-1", getUiTypographyClassName({ role: "supporting", tone: "muted" }))}>
            {t("capability.connector_richmail_prepare_intro")}
          </p>
          <dl className="mt-4 border-t border-(--divider-subtle-color) pt-3">
            <dt className={getUiTypographyClassName({ role: "metadata", tone: "muted", weight: "medium" })}>
              {t("capability.connector_richmail_settings_path_label")}
            </dt>
            <dd className={cn("mt-1 break-words", getUiTypographyClassName({ role: "supporting", tone: "default" }))}>
              {t("capability.connector_richmail_settings_path")}
            </dd>
          </dl>
          <p className={cn("mt-2", getUiTypographyClassName({ role: "supporting", tone: "default", weight: "medium" }))}>
            {t("capability.connector_richmail_enable_setting")}
          </p>
          <p className={cn(
            "mt-3",
            getUiTypographyClassName({ role: "supporting", tone: "muted" }),
          )}>
            {t("capability.connector_richmail_prepare_next")}
          </p>
        </UiPanel>
      ) : null}
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
      {mcpTools.supported ? (
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
