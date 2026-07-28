"use client";

/**
 * INPUT: 当前 DM 的 pending interaction 队列与响应动作。
 * OUTPUT: 原位替换输入壳内容的权限、计划确认或结构化问答组件。
 * POS: Composer 内唯一可操作的 DM 人工介入 surface。
 */
import { CircleHelp, ListChecks, ShieldCheck } from "lucide-react";
import { useRef, useState, type ReactNode } from "react";

import { PendingHumanQuestion } from "@/features/conversation/shared/message/blocks/question/pending-human-question";
import { ToolBlock } from "@/features/conversation/shared/message/blocks/tool/tool-block";
import type { ToolPermissionRequest } from "@/features/conversation/shared/message/blocks/tool/tool-block-types";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  PendingPermission,
  PermissionDecisionPayload,
  PermissionUpdate,
} from "@/types/conversation/interaction/permission";

import {
  buildComposerInteractionQueue,
  type ComposerInteractionKind,
} from "./composer-interaction-model";

export interface ComposerInteractionSurfaceProps {
  onResponse: (payload: PermissionDecisionPayload) => boolean;
  permissions: PendingPermission[];
  workspaceAgentId?: string | null;
}

const INTERACTION_ICON = {
  permission: ShieldCheck,
  plan: ListChecks,
  question: CircleHelp,
} as const;

export function ComposerInteractionSurface({
  onResponse,
  permissions,
  workspaceAgentId,
}: ComposerInteractionSurfaceProps) {
  const queue = buildComposerInteractionQueue(permissions);
  if (!queue.current || !queue.kind) {
    return null;
  }
  return (
    <ComposerInteractionRequest
      key={queue.current.request_id}
      kind={queue.kind}
      onResponse={onResponse}
      permission={queue.current}
      total={queue.total}
      workspaceAgentId={workspaceAgentId}
    />
  );
}

function ComposerInteractionRequest({
  kind,
  onResponse,
  permission,
  total,
  workspaceAgentId,
}: {
  kind: ComposerInteractionKind;
  onResponse: ComposerInteractionSurfaceProps["onResponse"];
  permission: PendingPermission;
  total: number;
  workspaceAgentId?: string | null;
}) {
  const { t } = useI18n();
  const respondingRef = useRef(false);
  const [isResponding, setIsResponding] = useState(false);
  const respond = (payload: PermissionDecisionPayload): boolean => {
    if (respondingRef.current) {
      return false;
    }
    respondingRef.current = true;
    setIsResponding(true);
    const sent = onResponse(payload);
    if (!sent) {
      respondingRef.current = false;
      setIsResponding(false);
    }
    return sent;
  };
  const Icon = INTERACTION_ICON[kind];

  return (
    <section
      aria-label={t(`composer.interaction_${kind}_label`)}
      aria-live="polite"
      className="message-cjk-font min-w-0"
      data-composer-interaction-kind={kind}
      data-composer-interaction-surface
      data-pending-interaction-request-id={permission.request_id}
    >
      <header className="flex min-h-11 items-center gap-2.5 border-b border-(--divider-subtle-color) px-4 py-2.5">
        <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary/8 text-primary">
          <Icon aria-hidden className="h-3.5 w-3.5" />
        </span>
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm font-medium text-(--text-strong)">
            {t(`composer.interaction_${kind}_label`)}
          </div>
          <div className="truncate text-xs text-(--text-soft)">
            {t("composer.interaction_replaces_input")}
          </div>
        </div>
        {total > 1 ? (
          <span
            className="shrink-0 rounded-full border border-(--divider-subtle-color) px-2 py-1 text-xs tabular-nums text-(--text-muted)"
            data-composer-interaction-queue
          >
            1 / {total}
          </span>
        ) : null}
      </header>
      <div className="soft-scrollbar max-h-[min(46vh,30rem)] overflow-y-auto px-3 py-2.5 [&_button]:min-h-11 [&_button]:min-w-11 sm:[&_button]:min-h-9 sm:[&_button]:min-w-0">
        <InteractionBody
          isResponding={isResponding}
          kind={kind}
          onResponse={respond}
          permission={permission}
          workspaceAgentId={workspaceAgentId}
        />
      </div>
    </section>
  );
}

function InteractionBody({
  isResponding,
  kind,
  onResponse,
  permission,
  workspaceAgentId,
}: {
  isResponding: boolean;
  kind: ComposerInteractionKind;
  onResponse: (payload: PermissionDecisionPayload) => boolean;
  permission: PendingPermission;
  workspaceAgentId?: string | null;
}): ReactNode {
  if (kind === "question") {
    return (
      <PendingHumanQuestion
        canRespond={!isResponding}
        onResponse={onResponse}
        permission={permission}
        readOnlyReason={isResponding ? "正在提交回应…" : undefined}
      />
    );
  }
  return (
    <ToolBlock
      interactionDisabled={isResponding}
      interactionDisabledReason={isResponding ? "正在提交确认…" : undefined}
      permissionRequest={createPermissionRequest(permission, onResponse)}
      status="waiting_permission"
      toolUse={{
        id: permission.tool_use_id?.trim()
          || `pending_${permission.request_id}`,
        input: permission.tool_input,
        name: permission.tool_name,
        type: "tool_use",
      }}
      workspaceAgentId={workspaceAgentId}
    />
  );
}

function createPermissionRequest(
  permission: PendingPermission,
  onResponse: (payload: PermissionDecisionPayload) => boolean,
): ToolPermissionRequest {
  const respond = (
    decision: PermissionDecisionPayload["decision"],
    updatedPermissions?: PermissionUpdate[],
  ) => {
    onResponse({
      decision,
      request_id: permission.request_id,
      updated_permissions: updatedPermissions,
    });
  };
  return {
    expires_at: permission.expires_at,
    on_allow: (updatedPermissions) => respond("allow", updatedPermissions),
    on_deny: (updatedPermissions) => respond("deny", updatedPermissions),
    request_id: permission.request_id,
    risk_label: permission.risk_label,
    risk_level: permission.risk_level,
    suggestions: permission.suggestions,
    summary: permission.summary,
    tool_input: permission.tool_input,
  };
}
