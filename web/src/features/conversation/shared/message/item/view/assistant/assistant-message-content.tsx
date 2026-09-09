/**
 * INPUT: Assistant direct/process/final 投影、活动状态、interaction owner 与请求切片。
 * OUTPUT: DM/Thread 的折叠工具段、Room 主 Feed 的等高单行活动摘要、固定位置的 final 正文、回复尾部生成文件汇总与唯一人工响应面。
 * POS: Assistant 正文、过程、终态与人工介入的纯视图编排层；Room 公区不消费具体工具过程。
 */
import { useMemo } from "react";
import { WorkspaceFileArtifactList } from "../../../blocks/artifact/workspace-file-artifacts";
import { useWorkspaceFileArtifactsFromContent } from "../../../blocks/artifact/workspace-file-artifact-utils";
import { AlertTriangle } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import type {
  ContentBlock,
  ToolUseContent,
} from "@/types/conversation/message/content";

import { type ContentProjection, shouldShowAssistantTimeline } from "../../message-item-projection";
import { getLocalizedToolActivityLabel } from "../../../tool-activity";
import { ProcessActivityIconStack } from "../../../ui/activity-icon";
import { LocalizedMessageActivityStatus } from "../message-activity-status";
import { ContentRenderer } from "../content/content-renderer";
import type {
  AssistantActivityState,
  AssistantContentEnvironment,
  AssistantDirectState,
  AssistantFinalState,
  AssistantPermissionState,
  AssistantProcessState,
} from "./assistant-message-model";
import { AssistantToolRuns } from "./assistant-dm-tool-runs";
import { AssistantProcessCallchain } from "./assistant-process-callchain";

const ROOM_RESULT_ACTIVITY_ALIGNMENT_CLASS_NAME =
  "px-0 [&_[data-message-activity-icon]]:justify-start";

interface AssistantMessageContentProps {
  activity: AssistantActivityState;
  direct: AssistantDirectState;
  environment: AssistantContentEnvironment;
  final: AssistantFinalState;
  permissions: AssistantPermissionState;
  process: AssistantProcessState;
  showMaxTokensWarning: boolean;
}

export function AssistantMessageContent({
  activity,
  direct,
  environment,
  final,
  permissions,
  process,
  showMaxTokensWarning,
}: AssistantMessageContentProps) {
  const filePresentation = useMemo(() => {
    const content = [
      ...process.projection.content,
      ...direct.projection.content,
      ...(final.visible && Array.isArray(final.content) ? final.content : []),
    ];
    const finalProjection = Array.isArray(final.content)
      ? withoutFileCards({ content: final.content, streamingIndexes: new Set(final.streamingIndexes) })
      : null;
    return {
      direct: { ...direct, projection: withoutFileCards(direct.projection) },
      process: { ...process, projection: withoutFileCards(process.projection) },
      final: finalProjection ? { ...final, content: finalProjection.content, streamingIndexes: finalProjection.streamingIndexes } : final,
      content,
    };
  }, [direct, process, final]);
  const artifacts = useWorkspaceFileArtifactsFromContent(filePresentation.content, environment.workspaceAgentId);
  return (
    <>
      <StandaloneActivity
        activity={activity}
        stableSlot={environment.mode === "room_result"}
      />
      <EmptyStreamStatus status={activity.emptyStreamStatus} />
      <AssistantDirectContent
        activity={activity}
        direct={filePresentation.direct}
        environment={environment}
        permissions={permissions}
        responseResumed={final.isStreaming}
        responseStreaming={final.isStreaming}
      />
      <AssistantProcessCallchain
        activity={activity}
        environment={environment}
        permissions={permissions}
        process={filePresentation.process}
      />
      <AssistantFinalContent
        activity={activity}
        environment={environment}
        final={filePresentation.final}
        permissions={permissions}
        showTrailingActivity={!direct.visible}
      />
      <RoomResultProcessActivity
        activity={activity}
        direct={filePresentation.direct}
        environment={environment}
      />
      <RoomResultTrailingActivity
        activity={activity}
        direct={filePresentation.direct}
        environment={environment}
        final={filePresentation.final}
      />
      <MaxTokensWarning visible={showMaxTokensWarning} />
      <WorkspaceFileArtifactList
        artifacts={artifacts}
        className="mt-3"
        onOpenWorkspaceFile={environment.onOpenWorkspaceFile}
        workspaceAgentId={environment.workspaceAgentId}
      />
    </>
  );
}

// Removing cards must reindex streaming markers alongside the remaining content.
function withoutFileCards(projection: ContentProjection): ContentProjection {
  const content: ContentBlock[] = [];
  const streamingIndexes = new Set<number>();
  projection.content.forEach((block, index) => {
    if (block.type === "workspace_file_artifact") return;
    if (projection.streamingIndexes.has(index)) streamingIndexes.add(content.length);
    content.push(block);
  });
  return { content, streamingIndexes };
}

function RoomResultProcessActivity({
  activity,
  direct,
  environment,
}: {
  activity: AssistantActivityState;
  direct: AssistantDirectState;
  environment: AssistantContentEnvironment;
}) {
  const { t } = useI18n();
  if (
    environment.mode !== "room_result"
    || !direct.visible
    || !activity.state
  ) {
    return null;
  }
  const runningTool = findLatestRunningTool(
    direct.projection.content,
    activity.state,
  );
  if (runningTool) {
    return (
      <div
        className="flex h-7 min-w-0 items-center gap-1.5 py-1 text-sm font-normal leading-5 text-primary"
        data-room-tool-activity
      >
        <ProcessActivityIconStack content={direct.projection.content} />
        <span
          aria-live="polite"
          className="nexus-live-tool-text min-w-0 flex-1 truncate"
        >
          {getLocalizedToolActivityLabel(
            runningTool.name,
            t,
            runningTool.input,
          )}
        </span>
      </div>
    );
  }
  return (
    <LocalizedMessageActivityStatus
      className={ROOM_RESULT_ACTIVITY_ALIGNMENT_CLASS_NAME}
      stableSlot
      state={activity.state}
      uniformTone
    />
  );
}

function findLatestRunningTool(
  content: readonly ContentBlock[],
  activityState: AssistantActivityState["state"],
): ToolUseContent | null {
  if (activityState !== "browsing" && activityState !== "executing") {
    return null;
  }
  const resolvedToolUseIds = new Set(content.flatMap((block) => (
    block.type === "tool_result" ? [block.tool_use_id] : []
  )));
  return content.findLast(
    (block): block is ToolUseContent => block.type === "tool_use"
      && !resolvedToolUseIds.has(block.id),
  ) ?? null;
}

function RoomResultTrailingActivity({
  activity,
  direct,
  environment,
  final,
}: {
  activity: AssistantActivityState;
  direct: AssistantDirectState;
  environment: AssistantContentEnvironment;
  final: AssistantFinalState;
}) {
  if (
    environment.mode !== "room_result"
    || activity.standalone
    || direct.visible
    || final.isStreaming
    || !activity.state
  ) {
    return null;
  }
  return (
    <LocalizedMessageActivityStatus
      className={ROOM_RESULT_ACTIVITY_ALIGNMENT_CLASS_NAME}
      label={activity.label}
      stableSlot
      state={activity.state}
    />
  );
}

function StandaloneActivity({
  activity,
  stableSlot,
}: {
  activity: AssistantActivityState;
  stableSlot: boolean;
}) {
  if (!activity.standalone || !activity.state) {
    return null;
  }
  return (
    <LocalizedMessageActivityStatus
      className={stableSlot
        ? ROOM_RESULT_ACTIVITY_ALIGNMENT_CLASS_NAME
        : "py-1"}
      label={activity.label}
      stableSlot={stableSlot}
      state={activity.state}
    />
  );
}

function AssistantDirectContent({
  activity,
  direct,
  environment,
  permissions,
  responseResumed,
  responseStreaming,
}: {
  activity: AssistantActivityState;
  direct: AssistantDirectState;
  environment: AssistantContentEnvironment;
  permissions: AssistantPermissionState;
  responseResumed: boolean;
  responseStreaming: boolean;
}) {
  if (!direct.visible) {
    return null;
  }
  if (environment.mode === "room_result") {
    return null;
  }
  if (environment.mode !== "dm_archived") {
    return (
      <AssistantToolRuns
        activity={activity}
        environment={environment}
        permissions={permissions}
        projection={direct.projection}
        responseResumed={responseResumed}
      />
    );
  }
  return (
    <ContentRenderer
      canRespondToPermissions={environment.canRespondToPermissions}
      content={direct.projection.content}
      fallbackActivityLabel={activity.label}
      fallbackActivityState={activity.state}
      hiddenToolNames={environment.hiddenToolNames}
      isStreaming={activity.showCursor && !responseStreaming}
      onOpenSubagentTask={environment.onOpenSubagentTask}
      onOpenWorkspaceFile={environment.onOpenWorkspaceFile}
      onPermissionResponse={environment.onPermissionResponse}
      pendingInteractionOwner={permissions.owner}
      pendingPermissionsByToolUseId={permissions.matchedByToolUseId}
      permissionReadOnlyReason={environment.permissionReadOnlyReason}
      showTimelineDots={shouldShowAssistantTimeline(environment.mode)}
      streamingBlockIndexes={direct.projection.streamingIndexes}
      unresolvedToolStatus={environment.unresolvedToolStatus}
      workspaceAgentId={environment.workspaceAgentId}
      agentMentionDirectory={environment.agentMentionDirectory}
      onOpenAgentContact={environment.onOpenAgentContact}
    />
  );
}

function AssistantFinalContent({
  activity,
  environment,
  final,
  permissions,
  showTrailingActivity,
}: {
  activity: AssistantActivityState;
  environment: AssistantContentEnvironment;
  final: AssistantFinalState;
  permissions: AssistantPermissionState;
  showTrailingActivity: boolean;
}) {
  if (!final.visible) {
    return null;
  }
  return (
    <ContentRenderer
      canRespondToPermissions={environment.canRespondToPermissions}
      className="nexus-chat-final-content mt-3.5 first:mt-0"
      content={final.content ?? []}
      fallbackActivityLabel={activity.label}
      fallbackActivityState={activity.state}
      isStreaming={final.isStreaming}
      onOpenSubagentTask={environment.onOpenSubagentTask}
      onOpenWorkspaceFile={environment.onOpenWorkspaceFile}
      onPermissionResponse={environment.onPermissionResponse}
      pendingInteractionOwner={permissions.owner}
      pendingPermissionsByToolUseId={permissions.matchedByToolUseId}
      permissionReadOnlyReason={environment.permissionReadOnlyReason}
      showTrailingActivity={showTrailingActivity}
      streamingBlockIndexes={final.streamingIndexes}
      unresolvedToolStatus={environment.unresolvedToolStatus}
      workspaceAgentId={environment.workspaceAgentId}
      agentMentions={final.mentions}
      agentMentionDirectory={environment.agentMentionDirectory}
      onOpenAgentContact={environment.onOpenAgentContact}
    />
  );
}

const EMPTY_STREAM_STATUS = {
  cancelled: {
    className: "text-xs italic text-(--text-soft)",
    labelKey: "message.stopped",
  },
  error: {
    className: "text-xs italic text-rose-500",
    labelKey: "message.failed",
  },
} as const;

function EmptyStreamStatus({
  status,
}: {
  status: AssistantActivityState["emptyStreamStatus"];
}) {
  const { t } = useI18n();
  if (!status) {
    return null;
  }
  const presentation = EMPTY_STREAM_STATUS[status];
  return <span className={presentation.className}>{t(presentation.labelKey)}</span>;
}

function MaxTokensWarning({ visible }: { visible: boolean }) {
  const { t } = useI18n();
  if (!visible) {
    return null;
  }
  return (
    <UiInlineNotice
      className="mt-2"
      icon={<AlertTriangle />}
      message={t("message.max_tokens_warning")}
      tone="warning"
    />
  );
}
