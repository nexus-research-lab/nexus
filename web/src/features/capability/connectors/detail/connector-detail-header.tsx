import type { ReactNode } from "react";
import {
  ArrowLeft,
  ChevronRight,
  KeyRound,
  Link2,
  Shield,
  Unplug,
} from "lucide-react";

import { UiButton } from "@/shared/ui/button/button";
import type { ConnectorDetail } from "@/types/capability/connector";

import { ConnectorIcon } from "../connector-icon";
import type {
  ConnectorDetailState,
  ConnectorOauthClientAction,
  ConnectorPrimaryAction,
} from "./connector-detail-model";

interface ConnectorActionContext {
  busy: boolean;
  detail: ConnectorDetail;
  onConfigureCredential: (detail: ConnectorDetail) => void;
  onConnect: (connectorId: string) => void;
  onDisconnect: (connectorId: string) => void;
}

const PRIMARY_ACTION: Record<
  ConnectorPrimaryAction,
  (context: ConnectorActionContext) => ReactNode
> = {
  connect: ({ busy, detail, onConnect }) => (
    <UiButton
      disabled={busy}
      onClick={() => onConnect(detail.connector_id)}
      size="sm"
      tone="primary"
      type="button"
      variant="solid"
    >
      <Link2 className="h-3.5 w-3.5" />
      添加到 Nexus
    </UiButton>
  ),
  "configure-credential": ({ busy, detail, onConfigureCredential }) => (
    <UiButton
      disabled={busy}
      onClick={() => onConfigureCredential(detail)}
      size="sm"
      tone="primary"
      type="button"
      variant="solid"
    >
      <KeyRound className="h-3.5 w-3.5" />
      配置凭证
    </UiButton>
  ),
  disconnect: ({ busy, detail, onDisconnect }) => (
    <UiButton
      disabled={busy}
      onClick={() => onDisconnect(detail.connector_id)}
      size="sm"
      type="button"
    >
      <Unplug className="h-3.5 w-3.5" />
      断开连接
    </UiButton>
  ),
  "coming-soon": () => (
    <UiButton disabled size="sm" type="button">
      即将推出
    </UiButton>
  ),
  unavailable: () => (
    <UiButton disabled size="sm" type="button">
      <Shield className="h-3.5 w-3.5" />
      后端未配置
    </UiButton>
  ),
  none: () => null,
};

interface OauthClientActionContext {
  busy: boolean;
  detail: ConnectorDetail;
  onConfigure: (detail: ConnectorDetail) => void;
}

const OAUTH_CLIENT_ACTION: Record<
  Exclude<ConnectorOauthClientAction, null>,
  (context: OauthClientActionContext) => ReactNode
> = {
  configure: ({ busy, detail, onConfigure }) => (
    <UiButton
      disabled={busy}
      onClick={() => onConfigure(detail)}
      size="sm"
      tone="primary"
      type="button"
      variant="solid"
    >
      <KeyRound className="h-3.5 w-3.5" />
      配置应用
    </UiButton>
  ),
  reconfigure: ({ busy, detail, onConfigure }) => (
    <UiButton
      disabled={busy}
      onClick={() => onConfigure(detail)}
      size="sm"
      type="button"
      variant="surface"
    >
      <KeyRound className="h-3.5 w-3.5" />
      配置应用
    </UiButton>
  ),
};

function ConnectorOauthClientButton({
  action,
  context,
}: {
  action: ConnectorOauthClientAction;
  context: OauthClientActionContext;
}) {
  if (!action) {
    return null;
  }
  return OAUTH_CLIENT_ACTION[action](context);
}

export function ConnectorDetailBreadcrumb({
  detail,
  onBack,
}: {
  detail: ConnectorDetail | null;
  onBack: () => void;
}) {
  return (
    <div className="flex items-center gap-2 text-[14px] text-(--text-muted)">
      <button
        className="inline-flex items-center gap-1 rounded-full px-2 py-1 font-medium transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_28%,transparent)]"
        onClick={onBack}
        type="button"
      >
        <ArrowLeft className="h-3.5 w-3.5" />
        连接器
      </button>
      {detail ? (
        <>
          <ChevronRight className="h-3.5 w-3.5 text-(--icon-muted)" />
          <span className="truncate font-medium text-(--text-strong)">
            {detail.title}
          </span>
        </>
      ) : null}
    </div>
  );
}

export function ConnectorDetailHeader({
  busy,
  detail,
  onConfigureCredential,
  onConfigureOauthClient,
  onConnect,
  onDisconnect,
  state,
}: ConnectorActionContext & {
  onConfigureOauthClient: (detail: ConnectorDetail) => void;
  state: ConnectorDetailState;
}) {
  const primaryAction = PRIMARY_ACTION[state.primaryAction]({
    busy,
    detail,
    onConfigureCredential,
    onConnect,
    onDisconnect,
  });
  return (
    <div className="flex flex-wrap items-start justify-between gap-4">
      <div className="flex min-w-0 items-start gap-4">
        <ConnectorIcon icon={detail.icon} size="lg" title={detail.title} />
        <div className="min-w-0">
          <h1 className="text-[24px] font-semibold tracking-[-0.035em] text-(--text-strong)">
            {detail.title}{" "}
            <span className="ml-2 font-normal text-(--text-muted)">App</span>
          </h1>
          <p className="mt-2 text-[15px] leading-6 text-(--text-muted)">
            {detail.description}
          </p>
        </div>
      </div>
      <div className="flex shrink-0 flex-wrap items-center justify-end gap-2">
        <ConnectorOauthClientButton
          action={state.oauthClientAction}
          context={{
            busy,
            detail,
            onConfigure: onConfigureOauthClient,
          }}
        />
        {primaryAction}
      </div>
    </div>
  );
}
