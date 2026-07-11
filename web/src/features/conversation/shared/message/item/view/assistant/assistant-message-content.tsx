import { AlertTriangle } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";

import type { MessageActivityState } from "../../activity/message-activity-state";
import { MessageActivityStatus } from "../message-activity-status";
import { ContentRenderer } from "../content/content-renderer";
import type {
  AssistantActivityState,
  AssistantContentEnvironment,
  AssistantDirectState,
  AssistantFinalState,
  AssistantPermissionState,
  AssistantProcessState,
} from "./assistant-message-model";
import { AssistantProcessCallchain } from "./assistant-process-callchain";
import { PendingPermissionList } from "./pending-permission-list";

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
  return (
    <>
      <StandaloneActivity activity={activity} />
      <EmptyStreamStatus status={activity.emptyStreamStatus} />
      <AssistantDirectContent
        activity={activity}
        direct={direct}
        environment={environment}
        permissions={permissions}
      />
      <AssistantProcessCallchain
        activity={activity}
        environment={environment}
        permissions={permissions}
        process={process}
      />
      <AssistantFinalContent
        activityState={activity.state}
        environment={environment}
        final={final}
      />
      <MaxTokensWarning visible={showMaxTokensWarning} />
      <PendingPermissionList
        canRespond={environment.canRespondToPermissions}
        mode={environment.mode}
        onResponse={environment.onPermissionResponse}
        permissions={permissions.unmatched}
        readOnlyReason={environment.permissionReadOnlyReason}
        workspaceAgentId={environment.workspaceAgentId}
      />
    </>
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
  return <MessageActivityStatus className="py-1" state={activity.state} />;
}

function AssistantDirectContent({
  activity,
  direct,
  environment,
  permissions,
}: {
  activity: AssistantActivityState;
  direct: AssistantDirectState;
  environment: AssistantContentEnvironment;
  permissions: AssistantPermissionState;
}) {
  if (!direct.visible) {
    return null;
  }
  return (
    <ContentRenderer
      canRespondToPermissions={environment.canRespondToPermissions}
      content={direct.projection.content}
      fallbackActivityState={activity.state}
      hiddenToolNames={environment.hiddenToolNames}
      isStreaming={activity.showCursor}
      onOpenWorkspaceFile={environment.onOpenWorkspaceFile}
      onPermissionResponse={environment.onPermissionResponse}
      pendingPermissionsByToolUseId={permissions.matchedByToolUseId}
      permissionReadOnlyReason={environment.permissionReadOnlyReason}
      showTimelineDots
      streamingBlockIndexes={direct.projection.streamingIndexes}
      workspaceAgentId={environment.workspaceAgentId}
    />
  );
}

function AssistantFinalContent({
  activityState,
  environment,
  final,
}: {
  activityState: MessageActivityState | null;
  environment: AssistantContentEnvironment;
  final: AssistantFinalState;
}) {
  if (!final.visible) {
    return null;
  }
  return (
    <ContentRenderer
      content={final.content ?? []}
      fallbackActivityState={activityState}
      isStreaming={final.isStreaming}
      onOpenWorkspaceFile={environment.onOpenWorkspaceFile}
      streamingBlockIndexes={final.streamingIndexes}
      workspaceAgentId={environment.workspaceAgentId}
    />
  );
}

const EMPTY_STREAM_STATUS = {
  cancelled: {
    className: "text-xs italic text-(--text-soft)",
    label: "已停止",
  },
  error: {
    className: "text-xs italic text-rose-500",
    label: "执行失败",
  },
} as const;

function EmptyStreamStatus({
  status,
}: {
  status: AssistantActivityState["emptyStreamStatus"];
}) {
  if (!status) {
    return null;
  }
  const presentation = EMPTY_STREAM_STATUS[status];
  return <span className={presentation.className}>{presentation.label}</span>;
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
