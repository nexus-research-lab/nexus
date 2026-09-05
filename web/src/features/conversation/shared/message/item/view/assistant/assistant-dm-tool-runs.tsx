/**
 * INPUT: DM/Room live 过程、当前 ToolUseSummary、final 恢复信号与人工交互工具集合。
 * OUTPUT: 执行中覆盖整段 process 的单行摘要，终态切换为中性审计入口；首层展开过程目录，各子项再独立展开详情。
 * POS: Assistant live 共用过程视图；权限、用户提问与生成式 UI 不进入折叠批次。
 */
"use client";

import { useEffect, useMemo, type RefObject } from "react";
import { Wrench } from "lucide-react";

import { useScrollAnchoredState } from "@/features/conversation/shared/timeline/scroll/use-scroll-anchored-state";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { isGenerativeUIWidgetToolName } from "@/lib/conversation/generative-ui";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { cn } from "@/shared/ui/class-name";
import type { ContentBlock } from "@/types/conversation/message/content";

import { WorkspaceFileArtifactList } from "../../../blocks/artifact/workspace-file-artifacts";
import { useWorkspaceFileArtifactsFromContent } from "../../../blocks/artifact/workspace-file-artifact-utils";
import {
  getLocalizedToolActivityLabel,
} from "../../../tool-activity";
import { MessageDetailScroll } from "../../../ui/message-rail";
import { ProcessActivityIconStack } from "../../../ui/activity-icon";
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
import { LocalizedMessageActivityStatus } from "../message-activity-status";
import { MessageDetailToggle } from "../../../ui/message-detail-toggle";
import type {
  AssistantActivityState,
  AssistantContentEnvironment,
  AssistantPermissionState,
} from "./assistant-message-model";
import {
  shouldShowAssistantTimeline,
  type ContentProjection,
} from "../../message-item-projection";

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
  const showTimeline = shouldShowAssistantTimeline(environment.mode);

  return (
    <div
      className={cn(
        "nexus-chat-block-stack min-w-0 space-y-0.5",
        showTimeline && TIMELINE_LINE_CLASS_NAME,
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
            showTimeline={showTimeline}
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
  showTimeline,
  streaming,
}: {
  activity: AssistantActivityState;
  environment: AssistantContentEnvironment;
  generatedFilesLabel: string;
  permissions: AssistantPermissionState;
  segment: ToolProcessSegment;
  showTimeline: boolean;
  streaming: boolean;
}) {
  if (segment.kind === "tool_run" && shouldCollapseToolRun(segment)) {
    return (
      <ToolRun
        activity={activity}
        environment={environment}
        generatedFilesLabel={generatedFilesLabel}
        permissions={permissions}
        segment={segment}
        showTimeline={showTimeline}
        streaming={streaming}
      />
    );
  }
  return (
    <TimelineBlock active={streaming} showRail={showTimeline}>
      <ToolProcessSegmentContent
        activity={activity}
        environment={environment}
        permissions={permissions}
        projection={segment.projection}
        showTrailingActivity
        streaming={streaming}
      />
    </TimelineBlock>
  );
}

function shouldCollapseToolRun(segment: ToolRunSegment): boolean {
  return segment.toolUseIds.length > 1
    || segment.projection.content.some((block) => (
      block.type === "thinking" && Boolean(block.thinking.trim())
    ));
}

function ToolRun({
  activity,
  environment,
  generatedFilesLabel,
  permissions,
  segment,
  showTimeline,
  streaming,
}: {
  activity: AssistantActivityState;
  environment: AssistantContentEnvironment;
  generatedFilesLabel: string;
  permissions: AssistantPermissionState;
  segment: ToolRunSegment;
  showTimeline: boolean;
  streaming: boolean;
}) {
  const { t } = useI18n();
  const expansion = useScrollAnchoredState(
    environment.mode === "room_thread"
      || environment.mode === "room_thread_process",
  );
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
  const phase = active ? "active" : segment.phase;
  const expanded = expansion.isOpen;
  const artifacts = useWorkspaceFileArtifactsFromContent(
    segment.projection.content,
  );
  const contentId = `${segment.id}-content`;
  const warning = phase === "error" || phase === "rejected";
  const summary = formatToolRunSummary(
    segment,
    phase,
    t,
  );

  return (
    <TimelineBlock active={streaming && active} showRail={showTimeline}>
      <div
        className="py-1.5"
        data-conversation-process-group-id={segment.id}
        data-tool-run-id={segment.id}
        data-tool-run-phase={phase}
        ref={expansion.anchorRef as RefObject<HTMLDivElement>}
      >
        <MessageDetailToggle
          aria-controls={contentId}
          data-timeline-anchor
          data-timeline-anchor-mode="box"
          expanded={expanded}
          leading={expanded ? (
            <Wrench className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
          ) : (
            <ProcessActivityIconStack content={segment.projection.content} />
          )}
          onClick={expansion.toggle}
          tone={warning ? "danger" : active ? "active" : "default"}
        >
          <span
            aria-live={active ? "polite" : undefined}
            className={cn(
              "min-w-0 flex-1 truncate",
              active && "nexus-live-tool-text",
            )}
            data-live-tool-text={active || undefined}
          >
            {summary}
          </span>
        </MessageDetailToggle>

        {expanded ? (
          <div data-tool-run-detail-list className="pt-1" id={contentId}>
            <MessageDetailScroll followContent={streaming && active}>
              <ToolProcessSegmentContent
                activity={activity}
                environment={environment}
                permissions={permissions}
                projection={segment.projection}
                showTrailingActivity={false}
                streaming={streaming && active}
              />
            </MessageDetailScroll>
          </div>
        ) : (
          <>
            <WorkspaceFileArtifactList
              artifacts={artifacts}
              className="ml-5 pt-1"
              label={generatedFilesLabel}
              onOpenWorkspaceFile={environment.onOpenWorkspaceFile}
            />
            {streaming && activity.state ? (
              <LocalizedMessageActivityStatus
                className="px-0 pt-1"
                state={activity.state}
              />
            ) : null}
          </>
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
  let statusKey: TranslationKey | null = null;
  if (phase === "error") {
    statusKey = "message.tool_run_failed";
  } else if (phase === "rejected") {
    statusKey = "message.tool_run_rejected";
  } else if (phase === "superseded") {
    statusKey = "message.tool_run_superseded";
  }
  const latestToolUse = segment.projection.content.findLast(
    (block): block is Extract<ContentBlock, { type: "tool_use" }> => (
      block.type === "tool_use"
    ),
  );
  const naturalSummary = segment.summaryText?.trim() || null;
  const activeFallback = latestToolUse
    ? getLocalizedToolActivityLabel(
        latestToolUse.name,
        t,
        latestToolUse.input,
      )
    : t("message.tool_run_active");
  const parts = [phase === "active"
    ? naturalSummary ?? activeFallback
    : t("message.tool_run_history")];
  if (
    statusKey
    && parts[0] !== t(statusKey)
  ) {
    parts.push(t(statusKey));
  }
  if (phase === "error" && segment.errorCount > 0) {
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
  permissions,
  projection,
  showTrailingActivity,
  streaming,
}: {
  activity: AssistantActivityState;
  environment: AssistantContentEnvironment;
  permissions: AssistantPermissionState;
  projection: ContentProjection;
  showTrailingActivity: boolean;
  streaming: boolean;
}) {
  return (
    <ContentRenderer
      canRespondToPermissions={environment.canRespondToPermissions}
      className="space-y-0.5"
      content={projection.content}
      defaultThinkingExpanded={false}
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
      showTrailingActivity={showTrailingActivity}
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
