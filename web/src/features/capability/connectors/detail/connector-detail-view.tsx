"use client";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { UiButton } from "@/shared/ui/button/button";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import { WORKSPACE_DETAIL_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import type {
  ConnectorDetail,
  ConnectorFeatureDetail,
} from "@/types/capability/connector";

import { getConnectorState } from "../model/connector-state-model";
import { ConnectorDetailContent } from "./connector-detail-content";
import {
  ConnectorDetailBreadcrumb,
  ConnectorDetailHeader,
} from "./connector-detail-header";
import { getConnectorFeatureDetails } from "./connector-detail-model";
import { ConnectorFeatureDialog } from "./connector-feature-dialog";

interface ConnectorDetailViewProps {
  busy: boolean;
  detail: ConnectorDetail | null;
  loading: boolean;
  onBack: () => void;
  onConfigureCredential: (detail: ConnectorDetail) => void;
  onConfigureOauthClient: (detail: ConnectorDetail) => void;
  onConnect: (connectorId: string) => void;
  onDisconnect: (connectorId: string) => void;
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

function ConnectorDetailLoading({
  detail,
  onBack,
}: Pick<ConnectorDetailViewProps, "detail" | "onBack">) {
  return (
    <div className={WORKSPACE_DETAIL_PAGE_CLASS_NAME}>
      <ConnectorDetailBreadcrumb detail={detail} onBack={onBack} />
      <UiStateBlock
        className="min-h-[420px]"
        size="md"
        title="加载连接器详情中..."
        variant="plain"
      />
    </div>
  );
}

function ConnectorDetailMissing({ onBack }: { onBack: () => void }) {
  return (
    <div className={WORKSPACE_DETAIL_PAGE_CLASS_NAME}>
      <ConnectorDetailBreadcrumb detail={null} onBack={onBack} />
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
    </div>
  );
}

/** 详情入口只协调资源状态和当前能力弹窗，不解释连接器动作。 */
export function ConnectorDetailView({
  busy,
  detail,
  loading,
  onBack,
  onConfigureCredential,
  onConfigureOauthClient,
  onConnect,
  onDisconnect,
}: ConnectorDetailViewProps) {
  const [selectedFeature, setSelectedFeature] = useResettableState<string | null>(
    null,
    detailIdentity(detail),
  );

  if (loading) {
    return <ConnectorDetailLoading detail={detail} onBack={onBack} />;
  }
  if (!detail) {
    return <ConnectorDetailMissing onBack={onBack} />;
  }

  const state = getConnectorState(detail);
  const features = getConnectorFeatureDetails(detail);
  return (
    <div className={WORKSPACE_DETAIL_PAGE_CLASS_NAME}>
      <ConnectorDetailBreadcrumb detail={detail} onBack={onBack} />
      <div className="pt-9">
        <ConnectorDetailHeader
          busy={busy}
          detail={detail}
          onConfigureCredential={onConfigureCredential}
          onConfigureOauthClient={onConfigureOauthClient}
          onConnect={onConnect}
          onDisconnect={onDisconnect}
          state={state}
        />
        <ConnectorDetailContent
          detail={detail}
          features={features}
          onSelectFeature={setSelectedFeature}
          state={state}
        />
      </div>
      <ConnectorFeatureDialog
        connectorTitle={detail.title}
        feature={selectedFeatureDetail(features, selectedFeature)}
        onClose={() => setSelectedFeature(null)}
      />
    </div>
  );
}
