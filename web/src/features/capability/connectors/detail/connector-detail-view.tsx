/**
 * INPUT: Connector 详情快照、资源状态、认证动作与返回目录命令。
 * OUTPUT: 使用统一能力详情内容轴的加载、失败、缺失和就绪页面。
 * POS: Connector 详情状态装配层；对象身份与动作投影归相邻 Header/model。
 */
"use client";

import type { ReactNode } from "react";

import { CapabilityDetailPage } from "@/features/capability/shared/capability-page-layout";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import type { ResourceFailure } from "@/lib/error-message";
import { UiButton } from "@/shared/ui/button/button";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  ConnectorDetail,
  ConnectorFeatureDetail,
} from "@/types/capability/connector";

import { getConnectorState } from "../model/connector-state-model";
import { ConnectorDetailContent } from "./connector-detail-content";
import {
  ConnectorDetailHeader,
} from "./connector-detail-header";
import { getConnectorFeatureDetails } from "./connector-detail-model";
import { ConnectorFeatureDialog } from "./connector-feature-dialog";
import { useConnectorMCPTools } from "./use-connector-mcp-tools";

interface ConnectorDetailViewProps {
  busy: boolean;
  detail: ConnectorDetail | null;
  failure: ResourceFailure | null;
  loading: boolean;
  onBack: () => void;
  onConfigureCredential: (detail: ConnectorDetail) => void;
  onConfigureOauthClient: (detail: ConnectorDetail) => void;
  onConnect: (connectorId: string) => void;
  onDisconnect: (connectorId: string) => void;
  onReplaceOauthClient: (detail: ConnectorDetail) => void;
  onRetry: () => void;
}

function detailIdentity(detail: ConnectorDetail | null): string | null {
  return detail ? detail.connector_id : null;
}

function selectedFeatureDetail(
  features: ConnectorFeatureDetail[],
  selectedFeature: string | null,
): ConnectorFeatureDetail | null {
  return features.find((feature) => feature.name === selectedFeature) || null;
}

function ConnectorDetailFrame({
  children,
  detail,
  onBack,
}: {
  children: ReactNode;
  detail: ConnectorDetail | null;
  onBack: () => void;
}) {
  const { t } = useI18n();
  return (
    <CapabilityDetailPage
      backLabel={t("capability.connectors")}
      currentTitle={detail?.title}
      onBack={onBack}
    >
      {children}
    </CapabilityDetailPage>
  );
}

function ConnectorDetailLoading({
  detail,
  onBack,
}: Pick<ConnectorDetailViewProps, "detail" | "onBack">) {
  return (
    <ConnectorDetailFrame detail={detail} onBack={onBack}>
      <UiStateBlock
        className="min-h-[420px]"
        size="md"
        title="加载连接器详情中..."
        variant="plain"
      />
    </ConnectorDetailFrame>
  );
}

function ConnectorDetailMissing({ onBack }: { onBack: () => void }) {
  return (
    <ConnectorDetailFrame detail={null} onBack={onBack}>
      <UiStateBlock
        actions={(
          <UiButton onClick={onBack} size="sm" type="button">
            返回连接器
          </UiButton>
        )}
        className="min-h-[420px]"
        size="md"
        title="连接器不存在"
        variant="plain"
      />
    </ConnectorDetailFrame>
  );
}

function ConnectorDetailFailure({
  onBack,
  onRetry,
}: {
  onBack: () => void;
  onRetry: () => void;
}) {
  const { t } = useI18n();
  return (
    <ConnectorDetailFrame detail={null} onBack={onBack}>
      <UiResourceState
        className="min-h-[420px]"
        impact={t("capability.connector_detail_load_failed_impact")}
        primaryAction={{
          label: t("capability.connector_detail_refresh"),
          onClick: onRetry,
        }}
        state="error"
        title={t("capability.connector_detail_load_failed_title")}
      />
    </ConnectorDetailFrame>
  );
}

/** 详情入口只协调资源状态和当前能力弹窗，不解释连接器动作。 */
export function ConnectorDetailView({
  busy,
  detail,
  failure,
  loading,
  onBack,
  onConfigureCredential,
  onConfigureOauthClient,
  onConnect,
  onDisconnect,
  onReplaceOauthClient,
  onRetry,
}: ConnectorDetailViewProps) {
  const { t } = useI18n();
  const [selectedFeature, setSelectedFeature] = useResettableState<string | null>(
    null,
    detailIdentity(detail),
  );
  const mcpTools = useConnectorMCPTools(detail);

  if (loading) {
    return <ConnectorDetailLoading detail={detail} onBack={onBack} />;
  }
  if (failure && !detail) {
    return (
      <ConnectorDetailFailure
        onBack={onBack}
        onRetry={onRetry}
      />
    );
  }
  if (!detail) {
    return <ConnectorDetailMissing onBack={onBack} />;
  }

  const state = getConnectorState(detail);
  const features = getConnectorFeatureDetails(detail);
  return (
    <ConnectorDetailFrame detail={detail} onBack={onBack}>
      {failure ? (
        <UiResourceState
          className="mt-4"
          impact={t("capability.connector_detail_stale_impact")}
          primaryAction={{
            label: t("capability.connector_detail_refresh"),
            onClick: onRetry,
          }}
          size="sm"
          state="error"
          title={t("capability.connector_detail_load_failed_title")}
        />
      ) : null}
      <div className="pt-5">
        <ConnectorDetailHeader
          busy={busy}
          detail={detail}
          onConfigureCredential={onConfigureCredential}
          onConfigureOauthClient={onConfigureOauthClient}
          onConnect={onConnect}
          onDisconnect={onDisconnect}
          onReplaceOauthClient={onReplaceOauthClient}
          state={state}
        />
        <ConnectorDetailContent
          detail={detail}
          features={features}
          mcpTools={mcpTools}
          onSelectFeature={setSelectedFeature}
          state={state}
        />
      </div>
      <ConnectorFeatureDialog
        connectorTitle={detail.title}
        feature={selectedFeatureDetail(features, selectedFeature)}
        onClose={() => setSelectedFeature(null)}
      />
    </ConnectorDetailFrame>
  );
}
