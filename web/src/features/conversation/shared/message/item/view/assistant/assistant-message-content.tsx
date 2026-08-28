/**
 * INPUT: Assistant direct/process/final 投影、活动状态、interaction owner 与请求切片。
 * OUTPUT: DM/Thread 的折叠工具段、Room 主 Feed 的单行活动摘要、固定位置的 final 正文与唯一人工响应面。
 * POS: Assistant 正文、过程、终态与人工介入的纯视图编排层；Room 公区不消费具体工具过程。
 */
import { AlertTriangle } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  ContentBlock,
  ToolUseContent,
} from "@/types/conversation/message/content";

import { shouldShowAssistantTimeline } from "../../message-item-projection";
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
  const { t } = useI18n();
  return (
    <>
      <StandaloneActivity activity={activity} />
      <EmptyStreamStatus status={activity.emptyStreamStatus} />
      <AssistantDirectContent
        activity={activity}
        direct={direct}
        environment={environment}
        generatedFilesLabel={t("message.generated_files")}
        permissions={permissions}
        responseResumed={final.isStreaming}
        responseStreaming={final.isStreaming}
      />
      <AssistantProcessCallchain
        activity={activity}
        environment={environment}
        generatedFilesLabel={t("message.generated_files")}
        permissions={permissions}
        process={process}
      />
      <AssistantFinalContent
        activity={activity}
        environment={environment}
        final={final}
        permissions={permissions}
        showTimeline={shouldShowAssistantTimeline(environment.mode)
          && (direct.visible || process.visible)}
        showTrailingActivity={!direct.visible}
      />
      <RoomResultProcessActivity
        activity={activity}
        direct={direct}
        environment={environment}
      />
      <RoomResultTrailingActivity
        activity={activity}
        direct={direct}
        environment={environment}
        final={final}
      />
      <MaxTokensWarning visible={showMaxTokensWarning} />
    </>
  );
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
        className="flex min-h-7 min-w-0 items-center gap-1.5 py-1 text-sm font-normal leading-5 text-primary"
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
      className="py-1"
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
      className="pt-1"
      label={activity.label}
      state={activity.state}
    />
  );
}

function StandaloneActivity({
  activity,
}: {
  activity: AssistantActivityState;
}) {
  if (!activity.standalone || !activity.state) {
    return null;
  }
  return (
    <LocalizedMessageActivityStatus
      className="py-1"
      label={activity.label}
      state={activity.state}
    />
  );
}

function AssistantDirectContent({
  activity,
  direct,
  environment,
  generatedFilesLabel,
  permissions,
  responseResumed,
  responseStreaming,
}: {
  activity: AssistantActivityState;
  direct: AssistantDirectState;
  environment: AssistantContentEnvironment;
  generatedFilesLabel: string;
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
        generatedFilesLabel={generatedFilesLabel}
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
  showTimeline,
  showTrailingActivity,
}: {
  activity: AssistantActivityState;
  environment: AssistantContentEnvironment;
  final: AssistantFinalState;
  permissions: AssistantPermissionState;
  showTimeline: boolean;
  showTrailingActivity: boolean;
}) {
  if (!final.visible) {
    return null;
  }
  return (
    <ContentRenderer
      canRespondToPermissions={environment.canRespondToPermissions}
      className="nexus-chat-final-content"
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
      showTimelineDots={showTimeline}
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
    <div className="mt-2 flex items-center gap-1.5 rounded-[8px] border border-[color:color-mix(in_srgb,var(--warning)_18%,transparent)] px-3 py-2 text-xs leading-5 text-(--warning)">
      <AlertTriangle className="h-3.5 w-3.5 shrink-0" />
      <span>{t("message.max_tokens_warning")}</span>
    </div>
  );
}
