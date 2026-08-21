/**
 * INPUT: DM/Room live 过程、当前 ToolUseSummary、final 恢复信号与人工交互工具集合。
 * OUTPUT: 以正文为边界持续更新的单行执行摘要；点开后显示完整过程与调用，生成物继续外露。
 * POS: Assistant live 共用过程视图；权限、用户提问与生成式 UI 不进入折叠批次。
 */
"use client";

import { useEffect, useMemo, type RefObject } from "react";
import { ChevronDown, ChevronRight, Wrench } from "lucide-react";

import { useScrollAnchoredState } from "@/features/conversation/shared/timeline/scroll/use-scroll-anchored-state";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { isGenerativeUIWidgetToolName } from "@/lib/conversation/generative-ui";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { cn } from "@/shared/ui/class-name";
import type { ContentBlock } from "@/types/conversation/message/content";

import { WorkspaceFileArtifactList } from "../../../blocks/artifact/workspace-file-artifacts";
import { useWorkspaceFileArtifactsFromContent } from "../../../blocks/artifact/workspace-file-artifact-utils";
import { getLocalizedToolTitle } from "../../../tool-activity";
import {
  projectToolRunSegments,
  type ToolProcessSegment,
  type ToolRunSegment,
} from "../../process/dm-tool-run-segments";
import { ContentRenderer } from "../content/content-renderer";
import {
  TIMELINE_LINE_CLASS_NAME,
  TimelineBlock,
} from "../content/content-renderer-timeline";
import type {
  AssistantActivityState,
  AssistantContentEnvironment,
  AssistantPermissionState,
} from "./assistant-message-model";
import type { ContentProjection } from "../../message-item-projection";

interface AssistantToolRunsProps {
  activity: AssistantActivityState;
  environment: AssistantContentEnvironment;
  generatedFilesLabel: string;
  permissions: AssistantPermissionState;
  projection: ContentProjection;
  responseResumed: boolean;
}

export function AssistantToolRuns({
  activity,
  environment,
  generatedFilesLabel,
  permissions,
  projection,
  responseResumed,
}: AssistantToolRunsProps) {
  const interactiveToolUseIds = useMemo(
    () => collectInteractiveToolUseIds(
      permissions.all,
      permissions.matchedByToolUseId,
      projection.content,
    ),
    [permissions.all, permissions.matchedByToolUseId, projection.content],
  );
  const segments = useMemo(
    () => projectToolRunSegments({
      interactiveToolUseIds,
      live: activity.showCursor,
      projection,
      responseResumed,
      toolUseSummary: activity.toolUseSummary,
    }),
    [
      activity.showCursor,
      interactiveToolUseIds,
      projection,
      responseResumed,
      activity.toolUseSummary,
    ],
  );

  return (
    <div
      className={cn(
        "nexus-chat-block-stack min-w-0 space-y-2.5",
        TIMELINE_LINE_CLASS_NAME,
      )}
      data-tool-run-list="true"
    >
      {segments.map((segment, index) => {
        const streaming = (
          activity.showCursor
          && !responseResumed
          && index === segments.length - 1
        );
        return (
          <ToolProcessSegmentView
            activity={activity}
            environment={environment}
            generatedFilesLabel={generatedFilesLabel}
            key={segment.id}
            permissions={permissions}
            segment={segment}
            streaming={streaming}
          />
        );
      })}
    </div>
  );
}

function ToolProcessSegmentView({
  activity,
  environment,
  generatedFilesLabel,
  permissions,
  segment,
  streaming,
}: {
  activity: AssistantActivityState;
  environment: AssistantContentEnvironment;
  generatedFilesLabel: string;
  permissions: AssistantPermissionState;
  segment: ToolProcessSegment;
  streaming: boolean;
}) {
  if (segment.kind === "tool_run") {
    return (
      <ToolRun
        activity={activity}
        environment={environment}
        generatedFilesLabel={generatedFilesLabel}
        permissions={permissions}
        segment={segment}
        streaming={streaming}
      />
    );
  }
  return (
    <TimelineBlock active={streaming}>
      <ToolProcessSegmentContent
        activity={activity}
        environment={environment}
        expandToolDetails={false}
        permissions={permissions}
        projection={segment.projection}
        streaming={streaming}
      />
    </TimelineBlock>
  );
}

function ToolRun({
  activity,
  environment,
  generatedFilesLabel,
  permissions,
  segment,
  streaming,
}: {
  activity: AssistantActivityState;
  environment: AssistantContentEnvironment;
  generatedFilesLabel: string;
  permissions: AssistantPermissionState;
  segment: ToolRunSegment;
  streaming: boolean;
}) {
  const { t } = useI18n();
  const expansion = useScrollAnchoredState(false);
  const [closedToolUseCount, setClosedToolUseCount] = useResettableState(
    0,
    segment.id,
  );
  useEffect(() => {
    if (segment.phase === "active") {
      return;
    }
    setClosedToolUseCount((currentCount) => (
      Math.max(currentCount, segment.toolUseIds.length)
    ));
  }, [
    segment.phase,
    segment.toolUseIds.length,
    setClosedToolUseCount,
  ]);
  const active = (
    segment.phase === "active"
    && (
      segment.unresolvedToolUseCount > 0
      || segment.toolUseIds.length > closedToolUseCount
    )
  );
  const phase = active
    ? "active"
    : segment.errorCount > 0
    ? "error"
    : segment.rejectedCount > 0
    ? "rejected"
    : segment.supersededCount > 0
    ? "superseded"
    : "complete";
  const expanded = expansion.isOpen;
  const artifacts = useWorkspaceFileArtifactsFromContent(
    segment.projection.content,
  );
  const contentId = `${segment.id}-content`;
  const error = segment.errorCount > 0 || segment.rejectedCount > 0;
  const summary = formatToolRunSummary(
    segment,
    phase,
    t,
  );

  return (
    <TimelineBlock active={streaming && active}>
      <div
        data-conversation-process-group-id={segment.id}
        data-tool-run-id={segment.id}
        data-tool-run-phase={phase}
        ref={expansion.anchorRef as RefObject<HTMLDivElement>}
      >
        <button
          aria-controls={contentId}
          aria-expanded={expanded}
          className={cn(
            "flex w-full items-center gap-2 py-1 text-left text-xs font-medium transition-colors duration-(--motion-duration-fast)",
            active
              ? "text-primary hover:text-primary"
              : "text-(--text-muted) hover:text-(--text-strong)",
            error && "text-rose-500 hover:text-rose-600",
          )}
          data-timeline-anchor
          data-timeline-anchor-mode="box"
          onClick={expansion.toggle}
          type="button"
        >
          <Wrench
            className={cn(
              "h-3.5 w-3.5 shrink-0",
              active && "animate-pulse",
            )}
          />
          <span className="min-w-0 flex-1 truncate">
            {summary}
          </span>
          {expanded ? (
            <ChevronDown className="h-3.5 w-3.5 shrink-0" />
          ) : (
            <ChevronRight className="h-3.5 w-3.5 shrink-0" />
          )}
        </button>

        {expanded ? (
          <div className="pt-1" id={contentId}>
            <ToolProcessSegmentContent
              activity={activity}
              environment={environment}
              expandToolDetails
              permissions={permissions}
              projection={segment.projection}
              streaming={streaming && active}
            />
          </div>
        ) : (
          <WorkspaceFileArtifactList
            artifacts={artifacts}
            className="ml-5 pt-1"
            label={generatedFilesLabel}
            onOpenWorkspaceFile={environment.onOpenWorkspaceFile}
          />
        )}
      </div>
    </TimelineBlock>
  );
}

function formatToolRunSummary(
  segment: ToolRunSegment,
  phase: ToolRunSegment["phase"],
  t: I18nContextValue["t"],
): string {
  const toolUseCount = segment.toolUseIds.length;
  const countKey = toolUseCount === 1
    ? "message.tool_run_count_one"
    : "message.tool_run_count_other";
  let statusKey: TranslationKey | null = null;
  if (phase === "active") {
    statusKey = "message.tool_run_active";
  } else if (phase === "error") {
    statusKey = "message.tool_run_failed";
  } else if (phase === "rejected") {
    statusKey = "message.tool_run_rejected";
  } else if (phase === "superseded") {
    statusKey = "message.tool_run_superseded";
  }
  const firstToolUse = segment.projection.content.find(
    (block) => block.type === "tool_use",
  );
  const fallbackLabel = toolUseCount === 1 && firstToolUse?.type === "tool_use"
    ? getLocalizedToolTitle(firstToolUse.name, t, firstToolUse.input)
    : t(countKey, { count: toolUseCount });
  const naturalSummary = segment.summaryText?.trim() || null;
  const parts = [naturalSummary ?? fallbackLabel];
  if (statusKey && !(phase === "active" && naturalSummary)) {
    parts.push(t(statusKey));
  }
  if (segment.errorCount > 0) {
    const errorKey = segment.errorCount === 1
      ? "message.tool_run_error_one"
      : "message.tool_run_error_other";
    parts.push(t(errorKey, { count: segment.errorCount }));
  }
  return parts.join(" · ");
}

function ToolProcessSegmentContent({
  activity,
  environment,
  expandToolDetails,
  permissions,
  projection,
  streaming,
}: {
  activity: AssistantActivityState;
  environment: AssistantContentEnvironment;
  expandToolDetails: boolean;
  permissions: AssistantPermissionState;
  projection: ContentProjection;
  streaming: boolean;
}) {
  return (
    <ContentRenderer
      canRespondToPermissions={environment.canRespondToPermissions}
      content={projection.content}
      defaultToolDetailsExpanded={expandToolDetails}
      fallbackActivityLabel={activity.label}
      fallbackActivityState={activity.state}
      hiddenToolNames={environment.hiddenToolNames}
      isStreaming={streaming}
      onOpenSubagentTask={environment.onOpenSubagentTask}
      onOpenWorkspaceFile={environment.onOpenWorkspaceFile}
      onPermissionResponse={environment.onPermissionResponse}
      pendingInteractionOwner={permissions.owner}
      pendingPermissionsByToolUseId={permissions.matchedByToolUseId}
      permissionReadOnlyReason={environment.permissionReadOnlyReason}
      streamingBlockIndexes={projection.streamingIndexes}
      unresolvedToolStatus={environment.unresolvedToolStatus}
      workspaceAgentId={environment.workspaceAgentId}
      agentMentionDirectory={environment.agentMentionDirectory}
      onOpenAgentContact={environment.onOpenAgentContact}
    />
  );
}

function collectInteractiveToolUseIds(
  permissions: AssistantPermissionState["all"],
  matchedByToolUseId: AssistantPermissionState["matchedByToolUseId"],
  content: readonly ContentBlock[],
): Set<string> {
  const toolUseIds = new Set<string>(
    matchedByToolUseId.keys(),
  );
  permissions.forEach((permission) => {
    if (permission.tool_use_id) {
      toolUseIds.add(permission.tool_use_id);
    }
  });
  content.forEach((block) => {
    if (
      block.type === "tool_use"
      && isGenerativeUIWidgetToolName(block.name)
    ) {
      toolUseIds.add(block.id);
    }
  });
  return toolUseIds;
}
