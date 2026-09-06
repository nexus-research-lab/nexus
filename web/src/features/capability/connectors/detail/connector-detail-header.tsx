/**
 * INPUT: Connector 对象、状态投影和认证/连接动作。
 * OUTPUT: Connector 身份说明、本地化动作和唯一 OAuth 应用配置按钮。
 * POS: Connector 详情对象投影；身份几何和二级页导航归 capability/shared。
 */
import type { ReactNode } from "react";
import {
  KeyRound,
  Link2,
  RefreshCcw,
  Shield,
  Unplug,
} from "lucide-react";

import { CapabilityDetailIdentity } from "@/features/capability/shared/capability-page-layout";
import { useI18n, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import type { ConnectorDetail } from "@/types/capability/connector";

import { ConnectorIcon } from "../connector-icon";
import type {
  ConnectorOauthClientAction,
  ConnectorPrimaryAction,
  ConnectorState,
} from "../model/connector-state-model";
import { canReplaceConnectorOauthClient } from "./connector-detail-model";

interface ConnectorActionContext {
  busy: boolean;
  detail: ConnectorDetail;
  onConfigureCredential: (detail: ConnectorDetail) => void;
  onConnect: (connectorId: string) => void;
  onDisconnect: (connectorId: string) => void;
}

const PRIMARY_ACTION: Record<
  ConnectorPrimaryAction,
  (context: ConnectorActionContext, t: I18nContextValue["t"]) => ReactNode
> = {
  connect: ({ busy, detail, onConnect }, t) => (
    <UiButton
      disabled={busy}
      onClick={() => onConnect(detail.connector_id)}
      size="sm"
      tone="primary"
      type="button"
      variant="solid"
    >
      <Link2 className="h-3.5 w-3.5" />
      {t("capability.connector_add_to_nexus")}
    </UiButton>
  ),
  "configure-credential": ({ busy, detail, onConfigureCredential }, t) => (
    <UiButton
      disabled={busy}
      onClick={() => onConfigureCredential(detail)}
      size="sm"
      tone="primary"
      type="button"
      variant="solid"
    >
      <KeyRound className="h-3.5 w-3.5" />
      {t("capability.connector_configure_credentials")}
    </UiButton>
  ),
  disconnect: ({ busy, detail, onDisconnect }, t) => (
    <UiButton
      disabled={busy}
      onClick={() => onDisconnect(detail.connector_id)}
      size="sm"
      type="button"
    >
      <Unplug className="h-3.5 w-3.5" />
      {t("capability.connector_disconnect")}
    </UiButton>
  ),
  "coming-soon": (_, t) => (
    <UiButton disabled size="sm" type="button">
      {t("capability.connector_card_coming_soon")}
    </UiButton>
  ),
  unavailable: (_, t) => (
    <UiButton disabled size="sm" type="button">
      <Shield className="h-3.5 w-3.5" />
      {t("capability.connector_service_unconfigured")}
    </UiButton>
  ),
  none: () => null,
};

interface OauthClientActionContext {
  busy: boolean;
  detail: ConnectorDetail;
  onConfigure: (detail: ConnectorDetail) => void;
}

function ConnectorOauthClientButton({
  action,
  context,
}: {
  action: ConnectorOauthClientAction;
  context: OauthClientActionContext;
}) {
  const { t } = useI18n();
  if (!action) {
    return null;
  }
  return (
    <UiButton disabled={context.busy} onClick={() => context.onConfigure(context.detail)}
      size="sm" tone={action === "configure" ? "primary" : undefined}
      variant={action === "configure" ? "solid" : "surface"}>
      <KeyRound className="h-3.5 w-3.5" />
      {t("capability.connector_configure_app")}
    </UiButton>
  );
}

export function ConnectorDetailHeader({
  busy,
  detail,
  onConfigureCredential,
  onConfigureOauthClient,
  onConnect,
  onDisconnect,
  onReplaceOauthClient,
  state,
}: ConnectorActionContext & {
  onConfigureOauthClient: (detail: ConnectorDetail) => void;
  onReplaceOauthClient: (detail: ConnectorDetail) => void;
  state: ConnectorState;
}) {
  const { t } = useI18n();
  const primaryAction = PRIMARY_ACTION[state.primaryAction]({
    busy,
    detail,
    onConfigureCredential,
    onConnect,
    onDisconnect,
  }, t);
  return (
    <CapabilityDetailIdentity
      actions={(
        <>
          {canReplaceConnectorOauthClient(detail) ? (
            <UiButton
              disabled={busy}
              onClick={() => onReplaceOauthClient(detail)}
              size="sm"
              type="button"
              variant="surface"
            >
              <RefreshCcw className="h-3.5 w-3.5" />
              {t("capability.connector_replace_feishu_app")}
            </UiButton>
          ) : null}
          <ConnectorOauthClientButton
            action={state.oauthClientAction}
            context={{
              busy,
              detail,
              onConfigure: onConfigureOauthClient,
            }}
          />
          {primaryAction}
        </>
      )}
      description={detail.description}
      leading={(
        <ConnectorIcon
          className="h-14 w-14 surface-radius-md"
          icon={detail.icon}
          size="lg"
          title={detail.title}
        />
      )}
      title={detail.title}
    />
  );
}
