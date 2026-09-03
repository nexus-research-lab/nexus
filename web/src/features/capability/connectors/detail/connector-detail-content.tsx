/**
 * INPUT: Connector 详情、连接状态、能力条目与文档地址。
 * OUTPUT: 当前对象的状态、能力与文档，不重复通用协议介绍。
 * POS: Connector 详情正文纯视图。
 */
import type { ReactNode } from "react";
import { Check, ChevronRight, ExternalLink, Power } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiButtonClassName } from "@/shared/ui/button/button-styles";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UiListRow } from "@/shared/ui/list/list-row";
import { UiPanel } from "@/shared/ui/panel";
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
import { getConnectorAuthLabel } from "./connector-detail-model";

const STATUS_BADGE: Record<
  ConnectorStatusTone,
  (state: ConnectorState) => ReactNode
> = {
  connected: () => (
    <UiBadge tone="success">
      <Check className="h-3.5 w-3.5" />
      已连接
    </UiBadge>
  ),
  "coming-soon": () => <UiBadge>即将推出</UiBadge>,
  unconfigured: (state) => (
    <UiBadge tone="warning">
      {state.oauthClientAction ? "待配置应用" : "后端未配置"}
    </UiBadge>
  ),
  disconnected: () => <UiBadge>未连接</UiBadge>,
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
      {STATUS_BADGE[state.status](state)}
      <UiBadge>{getConnectorAuthLabel(detail.auth_type)}</UiBadge>
      <UiBadge>{getConnectorCategoryLabel(detail.category, t)}</UiBadge>
      {detail.scopes.length > 0 ? (
        <UiBadge>{detail.scopes.length} 项权限范围</UiBadge>
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
  if (features.length === 0) {
    return null;
  }
  return (
    <section>
      <h2 className="mb-2 text-base font-medium text-(--text-strong)">
        包含内容
      </h2>
      <UiPanel
        className="divide-y divide-(--divider-subtle-color)"
        padding="none"
        radius="md"
        variant="card"
      >
        {features.map((feature) => (
          <UiListRow
            className="min-h-[56px] rounded-none"
            description={feature.description}
            key={feature.name}
            leading={(
              <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-(--divider-subtle-color) bg-(--surface-panel-background)">
                <Check className="h-4 w-4 text-(--icon-muted)" />
              </span>
            )}
            onClick={() => onSelect(feature.name)}
            right={<ChevronRight className="h-4 w-4 shrink-0 text-(--icon-muted)" />}
            title={feature.name}
          />
        ))}
      </UiPanel>
    </section>
  );
}

function ConnectorDocsLink({ url }: { url: string | undefined }) {
  if (!url) {
    return null;
  }
  return (
    <a
      className={getUiButtonClassName(
        { size: "sm", variant: "text" },
        "w-fit",
      )}
      href={url}
      rel="noopener noreferrer"
      target="_blank"
    >
      <ExternalLink className="h-3.5 w-3.5" />
      查看文档
    </a>
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
  const connected = detail.connection_state === "connected";
  const serverName = catalog?.server_title
    || catalog?.server_name
    || "连接后获取";
  return (
    <section className="border-y border-(--divider-subtle-color) py-5">
      <h2 className="text-base font-medium text-(--text-strong)">
        连接信息
      </h2>
      <dl className="mt-3 grid gap-x-8 gap-y-3 sm:grid-cols-2">
        <ConnectorFact label="服务地址" value={detail.mcp_server_url || "http://127.0.0.1:3100/mcp"} />
        <ConnectorFact label="传输" value="Streamable HTTP" />
        <ConnectorFact label="认证" value="RichMail 本机审批 · Bearer Token" />
        <ConnectorFact
          label="服务器"
          value={catalog?.server_version
            ? `${serverName} · ${catalog.server_version}`
            : serverName}
        />
        <ConnectorFact
          label="协议版本"
          value={catalog?.protocol_version || "连接后获取"}
        />
        <ConnectorFact
          label="Token"
          value={connected ? "已安全保存 · 通常 7 天有效" : "批准后由 RichMail 签发"}
        />
      </dl>
      {!connected ? (
        <div
          aria-label="RichMail 连接前准备"
          className="mt-5 surface-radius-md border border-[color:color-mix(in_srgb,var(--brand-action)_20%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--brand-action)_4%,transparent)] p-4"
          role="note"
        >
          <div className="flex items-start gap-3">
            <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[9px] bg-[color:color-mix(in_srgb,var(--brand-action)_11%,transparent)] text-(--brand-action)">
              <Power className="h-4 w-4" />
            </span>
            <div className="min-w-0">
              <p className="text-[11px] font-semibold tracking-[0.08em] text-(--brand-action)">
                连接前准备
              </p>
              <h3 className="mt-1 text-sm font-semibold text-(--text-strong)">
                请先在 RichMail 开启 Agent MCP 服务
              </h3>
              <p className="mt-1 text-xs leading-5 text-(--text-muted)">
                打开 RichMail 并保持后台运行，然后按照下面的路径完成设置。
              </p>
            </div>
          </div>

          <div className="mt-4 flex flex-col gap-3 border-t border-[color:color-mix(in_srgb,var(--brand-action)_14%,var(--divider-subtle-color))] pt-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="min-w-0">
              <p className="text-[11px] font-medium text-(--text-soft)">
                RichMail 设置路径
              </p>
              <p className="mt-1 text-xs font-medium leading-5 text-(--text-default)">
                设置 → Rwork → 智能体与能力 → 对外 MCP 服务
              </p>
            </div>
            <span className="inline-flex w-fit shrink-0 items-center gap-1.5 rounded-full border border-[color:color-mix(in_srgb,var(--brand-action)_22%,var(--divider-subtle-color))] bg-(--surface-panel-background) px-2.5 py-1 text-xs font-medium text-(--text-strong)">
              <Check className="h-3.5 w-3.5 text-(--brand-action)" />
              开启「启用 Agent MCP 服务」
            </span>
          </div>

          <p className="mt-3 text-xs leading-5 text-(--text-muted)">
            完成后返回 Nexus 开始连接，并在 RichMail 中批准本次授权。
          </p>
        </div>
      ) : null}
    </section>
  );
}

function ConnectorFact({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid grid-cols-[88px_minmax(0,1fr)] gap-3 text-sm">
      <dt className="text-(--text-soft)">{label}</dt>
      <dd className="min-w-0 break-words text-(--text-default)">{value}</dd>
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
          description="这些名称、说明和参数来自 RichMail 的 MCP tools/list，连接后可供对话选择。"
          failure={mcpTools.failure}
          loading={mcpTools.loading}
          onRetry={mcpTools.refresh}
          unavailableMessage="连接 RichMail 后即可读取并查看当前工具。"
        />
      ) : null}
      <ConnectorDocsLink url={detail.docs_url} />
    </div>
  );
}
