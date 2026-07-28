/**
 * INPUT: DM live direct 过程、当前活动状态、final 恢复信号与人工交互工具集合。
 * OUTPUT: 连续工具调用的单一时间线块；新执行段展开，叙事/final 边界后默认折叠。
 * POS: Assistant DM live 专用过程视图，不参与 Room 或权限响应面的所有权选择。
 */
"use client";

import { useEffect, useMemo, type RefObject } from "react";
import { ChevronDown, ChevronRight, Wrench } from "lucide-react";

import { useScrollAnchoredState } from "@/features/conversation/shared/timeline/scroll/use-scroll-anchored-state";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { cn } from "@/shared/ui/class-name";

import { WorkspaceFileArtifactList } from "../../../blocks/artifact/workspace-file-artifacts";
import { useWorkspaceFileArtifactsFromContent } from "../../../blocks/artifact/workspace-file-artifact-utils";
import {
  buildDmToolRunSummary,
  projectDmToolRunSegments,
  type DmProcessSegment,
  type DmToolRunSegment,
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

interface AssistantDmToolRunsProps {
  activity: AssistantActivityState;
  environment: AssistantContentEnvironment;
  permissions: AssistantPermissionState;
  projection: ContentProjection;
  responseResumed: boolean;
}

export function AssistantDmToolRuns({
  activity,
  environment,
  permissions,
  projection,
  responseResumed,
}: AssistantDmToolRunsProps) {
  const interactiveToolUseIds = useMemo(
    () => collectInteractiveToolUseIds(
      permissions.all,
      permissions.matchedByToolUseId,
    ),
    [permissions.all, permissions.matchedByToolUseId],
  );
  const segments = useMemo(
    () => projectDmToolRunSegments({
      interactiveToolUseIds,
      live: activity.showCursor,
      projection,
      responseResumed,
    }),
    [
      activity.showCursor,
      interactiveToolUseIds,
      projection,
      responseResumed,
    ],
  );

  return (
    <div
      className={cn(
        "nexus-chat-block-stack min-w-0 space-y-2.5",
        TIMELINE_LINE_CLASS_NAME,
      )}
      data-dm-tool-run-list="true"
    >
      {segments.map((segment, index) => {
        const streaming = (
          activity.showCursor
          && !responseResumed
          && index === segments.length - 1
        );
        return (
          <DmProcessSegmentView
            activity={activity}
            environment={environment}
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

function DmProcessSegmentView({
  activity,
  environment,
  permissions,
  segment,
  streaming,
}: {
  activity: AssistantActivityState;
  environment: AssistantContentEnvironment;
  permissions: AssistantPermissionState;
  segment: DmProcessSegment;
  streaming: boolean;
}) {
  if (segment.kind === "tool_run") {
    return (
      <DmToolRun
        activity={activity}
        environment={environment}
        permissions={permissions}
        segment={segment}
        streaming={streaming}
      />
    );
  }
  return (
    <TimelineBlock active={streaming}>
      <DmProcessSegmentContent
        activity={activity}
        environment={environment}
        permissions={permissions}
        projection={segment.projection}
        streaming={streaming}
      />
    </TimelineBlock>
  );
}

function DmToolRun({
  activity,
  environment,
  permissions,
  segment,
  streaming,
}: {
  activity: AssistantActivityState;
  environment: AssistantContentEnvironment;
  permissions: AssistantPermissionState;
  segment: DmToolRunSegment;
  streaming: boolean;
}) {
  const expansion = useScrollAnchoredState(false);
  const [closedToolUseCount, setClosedToolUseCount] = useResettableState(
    0,
    segment.id,
  );
  useEffect(() => {
    if (segment.phase !== "complete") {
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
  const phase = active ? "active" : "complete";
  const expanded = active || expansion.isOpen;
  const artifacts = useWorkspaceFileArtifactsFromContent(
    segment.projection.content,
  );
  const contentId = `${segment.id}-content`;
  const error = segment.errorCount > 0;
  const summary = buildDmToolRunSummary(
    segment.toolUseIds.length,
    segment.errorCount,
    phase,
  );

  return (
    <TimelineBlock active={streaming && active}>
      <div
        data-conversation-process-group-id={segment.id}
        data-dm-tool-run-id={segment.id}
        data-dm-tool-run-phase={phase}
        ref={expansion.anchorRef as RefObject<HTMLDivElement>}
      >
        <button
          aria-controls={contentId}
          aria-disabled={active}
          aria-expanded={expanded}
          className={cn(
            "flex w-full items-center gap-2 py-1 text-left text-xs font-medium transition-colors duration-(--motion-duration-fast)",
            active
              ? "cursor-default text-primary"
              : "text-(--text-muted) hover:text-(--text-strong)",
            error && "text-rose-500 hover:text-rose-600",
          )}
          data-timeline-anchor
          data-timeline-anchor-mode="box"
          onClick={active ? undefined : expansion.toggle}
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
            <DmProcessSegmentContent
              activity={activity}
              environment={environment}
              permissions={permissions}
              projection={segment.projection}
              streaming={streaming && active}
            />
          </div>
        ) : (
          <WorkspaceFileArtifactList
            artifacts={artifacts}
            className="ml-5 pt-1"
            label="生成文件"
            onOpenWorkspaceFile={environment.onOpenWorkspaceFile}
          />
        )}
      </div>
    </TimelineBlock>
  );
}

function DmProcessSegmentContent({
  activity,
  environment,
  permissions,
  projection,
  streaming,
}: {
  activity: AssistantActivityState;
  environment: AssistantContentEnvironment;
  permissions: AssistantPermissionState;
  projection: ContentProjection;
  streaming: boolean;
}) {
  return (
    <ContentRenderer
      canRespondToPermissions={environment.canRespondToPermissions}
      content={projection.content}
      fallbackActivityState={activity.state}
      hiddenToolNames={environment.hiddenToolNames}
      isStreaming={streaming}
      onOpenWorkspaceFile={environment.onOpenWorkspaceFile}
      onPermissionResponse={environment.onPermissionResponse}
      pendingInteractionOwner={permissions.owner}
      pendingPermissionsByToolUseId={permissions.matchedByToolUseId}
      permissionReadOnlyReason={environment.permissionReadOnlyReason}
      streamingBlockIndexes={projection.streamingIndexes}
      workspaceAgentId={environment.workspaceAgentId}
    />
  );
}

function collectInteractiveToolUseIds(
  permissions: AssistantPermissionState["all"],
  matchedByToolUseId: AssistantPermissionState["matchedByToolUseId"],
): Set<string> {
  const toolUseIds = new Set<string>(
    matchedByToolUseId.keys(),
  );
  permissions.forEach((permission) => {
    if (permission.tool_use_id) {
      toolUseIds.add(permission.tool_use_id);
    }
  });
  return toolUseIds;
}
