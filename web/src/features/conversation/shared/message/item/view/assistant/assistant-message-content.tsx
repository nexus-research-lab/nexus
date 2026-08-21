/**
 * INPUT: Assistant direct/process/final 投影、活动状态、interaction owner 与请求切片。
 * OUTPUT: DM live 连续工具段、live/terminal 固定位置的 final 正文，以及只在 owner 轨道挂载一次的人工响应面。
 * POS: Assistant 正文、过程、终态与人工介入的纯视图编排层。
 */
import { AlertTriangle } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";

import { shouldShowAssistantTimeline } from "../../message-item-projection";
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
import { AssistantDmToolRuns } from "./assistant-dm-tool-runs";
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
      />
      <RoomResultTrailingActivity
        activity={activity}
        environment={environment}
        final={final}
      />
      <MaxTokensWarning visible={showMaxTokensWarning} />
    </>
  );
}

function RoomResultTrailingActivity({
  activity,
  environment,
  final,
}: {
  activity: AssistantActivityState;
  environment: AssistantContentEnvironment;
  final: AssistantFinalState;
}) {
  if (
    environment.mode !== "room_result"
    || activity.standalone
    || final.isStreaming
    || !activity.state
  ) {
    return null;
  }
  return <LocalizedMessageActivityStatus className="pt-1" label={activity.label} state={activity.state} />;
}

function StandaloneActivity({
  activity,
}: {
  activity: AssistantActivityState;
}) {
  if (!activity.standalone || !activity.state) {
    return null;
  }
  return <LocalizedMessageActivityStatus className="py-1" label={activity.label} state={activity.state} />;
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
  if (environment.mode === "dm_live") {
    return (
      <AssistantDmToolRuns
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
}: {
  activity: AssistantActivityState;
  environment: AssistantContentEnvironment;
  final: AssistantFinalState;
  permissions: AssistantPermissionState;
}) {
  if (!final.visible) {
    return null;
  }
  return (
    <ContentRenderer
      canRespondToPermissions={environment.canRespondToPermissions}
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
