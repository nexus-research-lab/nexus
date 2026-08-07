"use client";

/**
 * INPUT: 当前 DM/Room 的 pending interaction 队列与响应动作。
 * OUTPUT: 原位替换输入壳内容的权限、计划确认或结构化问答组件。
 * POS: Composer 内唯一可操作的会话人工介入 surface。
 */
import { MessageSquare } from "lucide-react";
import { useRef, useState, type ReactNode } from "react";

import { PendingHumanQuestion } from "@/features/conversation/shared/message/blocks/question/pending-human-question";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";

import {
  buildComposerInteractionQueue,
  type ComposerInteractionKind,
} from "./composer-interaction-model";
import { ComposerPermissionSurface } from "./composer-permission-surface";

export interface ComposerInteractionSurfaceProps {
  agentAvatarMap?: Readonly<Record<string, string | null>>;
  agentNameMap?: Readonly<Record<string, string>>;
  fallbackAgentId?: string | null;
  onResponse: (payload: PermissionDecisionPayload) => boolean;
  permissions: PendingPermission[];
}

export function ComposerInteractionSurface({
  agentAvatarMap,
  agentNameMap,
  fallbackAgentId,
  onResponse,
  permissions,
}: ComposerInteractionSurfaceProps) {
  const queue = buildComposerInteractionQueue(permissions);
  if (!queue.current || !queue.kind) {
    return null;
  }
  const requester = resolveRequester(
    queue.current,
    fallbackAgentId,
    agentAvatarMap,
    agentNameMap,
  );
  return (
    <ComposerInteractionRequest
      key={queue.current.request_id}
      kind={queue.kind}
      onResponse={onResponse}
      permission={queue.current}
      requester={requester}
      total={queue.total}
    />
  );
}

function ComposerInteractionRequest({
  kind,
  onResponse,
  permission,
  requester,
  total,
}: {
  kind: ComposerInteractionKind;
  onResponse: ComposerInteractionSurfaceProps["onResponse"];
  permission: PendingPermission;
  requester: InteractionRequester;
  total: number;
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

  return (
    <section
      aria-label={t(`composer.interaction_${kind}_label`)}
      aria-live="polite"
      className="message-cjk-font min-w-0"
      data-composer-interaction-agent-id={requester.id}
      data-composer-interaction-kind={kind}
      data-composer-interaction-surface
      data-pending-interaction-request-id={permission.request_id}
    >
      <div
        className={cn(
          "soft-scrollbar max-h-[min(46vh,30rem)] overflow-y-auto [&_button]:min-h-11 [&_button]:min-w-11 sm:[&_button]:min-w-0",
          kind === "question"
            ? "p-3 sm:p-4 sm:[&_button]:min-h-8"
            : "p-4 sm:p-5 sm:[&_button]:min-h-8",
        )}
      >
        <InteractionBody
          isResponding={isResponding}
          kind={kind}
          onResponse={respond}
          permission={permission}
          requester={requester}
          total={total}
        />
      </div>
    </section>
  );
}

interface InteractionRequester {
  avatar?: string | null;
  id?: string;
  name?: string;
}

function resolveRequester(
  permission: PendingPermission,
  fallbackAgentId?: string | null,
  agentAvatarMap?: Readonly<Record<string, string | null>>,
  agentNameMap?: Readonly<Record<string, string>>,
): InteractionRequester {
  const agentId = permission.agent_id?.trim() || fallbackAgentId?.trim();
  return agentId
    ? {
      avatar: agentAvatarMap?.[agentId],
      id: agentId,
      name: agentNameMap?.[agentId],
    }
    : {};
}

function InteractionBody({
  isResponding,
  kind,
  onResponse,
  permission,
  requester,
  total,
}: {
  isResponding: boolean;
  kind: ComposerInteractionKind;
  onResponse: (payload: PermissionDecisionPayload) => boolean;
  permission: PendingPermission;
  requester: InteractionRequester;
  total: number;
}): ReactNode {
  if (kind === "question") {
    return (
      <div className="space-y-2">
        <QuestionRequester requester={requester} total={total} />
        <PendingHumanQuestion
          canRespond={!isResponding}
          onResponse={onResponse}
          permission={permission}
        />
      </div>
    );
  }
  return (
    <ComposerPermissionSurface
      interactionDisabled={isResponding}
      kind={kind}
      onResponse={onResponse}
      permission={permission}
      requesterAvatar={requester.avatar}
      requesterName={requester.name}
      total={total}
    />
  );
}

function QuestionRequester({
  requester,
  total,
}: {
  requester: InteractionRequester;
  total: number;
}) {
  const { t } = useI18n();
  return (
    <div className="flex min-w-0 items-center gap-2 text-sm text-(--text-muted)">
      {requester.name ? (
        <>
          <UiAgentAvatar
            avatar={requester.avatar}
            data-composer-interaction-requester
            name={requester.name}
            size="xs"
          />
          <span className="truncate font-medium text-(--text-strong)">
            {requester.name}
          </span>
          <span aria-hidden className="text-(--text-soft)">·</span>
        </>
      ) : null}
      <MessageSquare
        aria-hidden
        className="h-4 w-4 shrink-0 text-(--icon-muted)"
      />
      <span className="truncate">
        {t("composer.question_request_title")}
      </span>
      {total > 1 ? (
        <span
          className="ml-auto shrink-0 text-xs tabular-nums text-(--text-soft)"
          data-composer-interaction-queue
        >
          1 / {total}
        </span>
      ) : null}
    </div>
  );
}
