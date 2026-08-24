import type { RefObject } from "react";
import { ChevronDown, ChevronRight, Wrench } from "lucide-react";

import type {
  ContentBlock,
  WorkspaceFileArtifactContent,
} from "@/types/conversation/message/content";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";

import { WorkspaceFileArtifactList } from "../../../blocks/artifact/workspace-file-artifacts";
import { useWorkspaceFileArtifactsFromContent } from "../../../blocks/artifact/workspace-file-artifact-utils";
import { getLocalizedToolTitle } from "../../../tool-activity";
import { shouldShowAssistantTimeline } from "../../message-item-projection";
import type {
  ProcessSummaryDetail,
  ProcessSummaryMetricKind,
  ProcessSummaryProjection,
} from "../../process/message-process-summary";
import { ContentRenderer } from "../content/content-renderer";
import type {
  AssistantActivityState,
  AssistantContentEnvironment,
  AssistantPermissionState,
  AssistantProcessState,
} from "./assistant-message-model";

const EMPTY_CONTENT_BLOCKS: ContentBlock[] = [];

interface AssistantProcessCallchainProps {
  activity: AssistantActivityState;
  environment: AssistantContentEnvironment;
  generatedFilesLabel: string;
  permissions: AssistantPermissionState;
  process: AssistantProcessState;
}

export function AssistantProcessCallchain({
  activity,
  environment,
  generatedFilesLabel,
  permissions,
  process,
}: AssistantProcessCallchainProps) {
  const collapsedFileArtifacts = useWorkspaceFileArtifactsFromContent(
    selectCollapsedProcessContent(process),
  );

  if (!process.visible) {
    return null;
  }

  return (
    <div ref={process.anchorRef as RefObject<HTMLDivElement>}>
      <ProcessToggleButton process={process} />
      <CollapsedProcessArtifacts
        artifacts={collapsedFileArtifacts}
        label={generatedFilesLabel}
        onOpenWorkspaceFile={environment.onOpenWorkspaceFile}
        visible={!process.expanded}
      />
      <ExpandedProcessContent
        activity={activity}
        environment={environment}
        permissions={permissions}
        process={process}
        visible={process.expanded}
      />
    </div>
  );
}

function selectCollapsedProcessContent(
  process: AssistantProcessState,
): ContentBlock[] {
  const shouldCollectArtifacts = process.visible && !process.expanded;
  return shouldCollectArtifacts
    ? process.projection.content
    : EMPTY_CONTENT_BLOCKS;
}

function ProcessToggleButton({ process }: { process: AssistantProcessState }) {
  const { t } = useI18n();
  return (
    <button
      className="flex w-full items-center gap-2 py-1.5 text-left text-(--text-muted) transition-colors duration-(--motion-duration-fast) hover:text-(--text-strong)"
      onClick={process.toggle}
      type="button"
    >
      <Wrench className="h-3 w-3 shrink-0 text-(--icon-muted)" />
      <div className="min-w-0 flex-1 truncate text-compact font-medium text-(--text-muted)">
        {formatProcessSummary(process.summary, t)}
      </div>
      <ProcessExpansionIcon expanded={process.expanded} />
    </button>
  );
}

const PROCESS_METRIC_KEYS: Record<
  ProcessSummaryMetricKind,
  { one: TranslationKey; other: TranslationKey }
> = {
  action: {
    one: "message.process_action_one",
    other: "message.process_action_other",
  },
  error: {
    one: "message.process_error_one",
    other: "message.process_error_other",
  },
  guidance: {
    one: "message.process_guidance_one",
    other: "message.process_guidance_other",
  },
  thinking: {
    one: "message.process_thinking_one",
    other: "message.process_thinking_other",
  },
};

function formatProcessSummary(
  summary: ProcessSummaryProjection,
  t: I18nContextValue["t"],
): string {
  if (summary.kind === "waiting_permission") {
    return t("message.process_waiting_permission");
  }
  const metricText = summary.metrics.map(({ count, kind }) => {
    const keys = PROCESS_METRIC_KEYS[kind];
    return t(count === 1 ? keys.one : keys.other, { count });
  });
  const overview = metricText.length > 0
    ? metricText.join(" · ")
    : t("message.process_view");
  if (!summary.latestDetail) {
    return overview;
  }
  return t("message.process_latest", {
    detail: formatProcessDetail(summary.latestDetail, t),
    summary: overview,
  });
}

function formatProcessDetail(
  detail: ProcessSummaryDetail,
  t: I18nContextValue["t"],
): string {
  if (detail.kind === "background_task") {
    return t("message.process_background_task");
  }
  if (detail.kind === "text") {
    return detail.text;
  }
  const title = getLocalizedToolTitle(detail.toolName, t);
  return detail.detail
    ? t("message.process_tool_detail", { detail: detail.detail, title })
    : title;
}

function ProcessExpansionIcon({ expanded }: { expanded: boolean }) {
  const Icon = expanded ? ChevronDown : ChevronRight;
  return (
    <div className="text-(--icon-muted)">
      <Icon className="h-3.5 w-3.5" />
    </div>
  );
}

function CollapsedProcessArtifacts({
  artifacts,
  label,
  onOpenWorkspaceFile,
  visible,
}: {
  artifacts: WorkspaceFileArtifactContent[];
  label: string;
  onOpenWorkspaceFile?: (path: string) => void;
  visible: boolean;
}) {
  if (!visible) {
    return null;
  }
  return (
    <WorkspaceFileArtifactList
      artifacts={artifacts}
      className="ml-5 pb-1"
      label={label}
      onOpenWorkspaceFile={onOpenWorkspaceFile}
    />
  );
}

function ExpandedProcessContent({
  activity,
  environment,
  permissions,
  process,
  visible,
}: {
  activity: AssistantActivityState;
  environment: AssistantContentEnvironment;
  permissions: AssistantPermissionState;
  process: AssistantProcessState;
  visible: boolean;
}) {
  if (!visible) {
    return null;
  }
  return (
    <div className="pt-1">
      <ContentRenderer
        canRespondToPermissions={environment.canRespondToPermissions}
        className="ml-1"
        content={process.projection.content}
        fallbackActivityLabel={activity.label}
        fallbackActivityState={activity.state}
        hiddenToolNames={environment.hiddenToolNames}
        isStreaming={activity.showCursor}
        onOpenSubagentTask={environment.onOpenSubagentTask}
        onOpenWorkspaceFile={environment.onOpenWorkspaceFile}
        onPermissionResponse={environment.onPermissionResponse}
        pendingInteractionOwner={permissions.owner}
        pendingPermissionsByToolUseId={permissions.matchedByToolUseId}
        permissionReadOnlyReason={environment.permissionReadOnlyReason}
        showTimelineDots={shouldShowAssistantTimeline(environment.mode)}
        streamingBlockIndexes={process.projection.streamingIndexes}
        unresolvedToolStatus={environment.unresolvedToolStatus}
        workspaceAgentId={environment.workspaceAgentId}
      />
    </div>
  );
}
