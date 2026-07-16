import type { ReactNode } from "react";
import { Check, ChevronRight, ExternalLink } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiButtonClassName } from "@/shared/ui/button/button-styles";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import { UiListRow } from "@/shared/ui/list/list-row";
import { UiPanel } from "@/shared/ui/panel";
import type {
  ConnectorDetail,
  ConnectorFeatureDetail,
} from "@/types/capability/connector";

import { getConnectorCategoryLabel } from "../catalog/connectors-categories";
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
  if (!error) {
    return null;
  }
  return (
    <UiStateBlock
      description={error}
      size="sm"
      title="配置不可用"
      tone="danger"
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
      <h2 className="mb-2 text-[15px] font-medium text-(--text-strong)">
        包含内容
      </h2>
      <UiPanel
        className="divide-y divide-(--divider-subtle-color)"
        padding="none"
        radius="md"
        variant="inset"
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

export function ConnectorDetailContent({
  detail,
  features,
  onSelectFeature,
  state,
}: {
  detail: ConnectorDetail;
  features: ConnectorFeatureDetail[];
  onSelectFeature: (featureName: string) => void;
  state: ConnectorState;
}) {
  return (
    <div className="mt-6 space-y-5">
      <p className="text-[13px] leading-5 text-(--text-default)">
        连接后，Agent 会通过安全的 MCP 协议访问此应用。你可以在需要时断开连接，OAuth 类型连接器也可以在原应用侧撤销授权。
      </p>
      <ConnectorStatusBadges detail={detail} state={state} />
      <ConnectorConfigurationError error={state.configurationError} />
      <ConnectorFeatureList features={features} onSelect={onSelectFeature} />
      <ConnectorDocsLink url={detail.docs_url} />
    </div>
  );
}
