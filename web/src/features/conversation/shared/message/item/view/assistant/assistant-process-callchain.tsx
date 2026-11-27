// INPUT: Archived process, assistant content environment and disclosure state.
// OUTPUT: Process disclosure; generated-file summaries are owned by the reply footer.
// POS: Archived Assistant process orchestration; file cards own action eligibility.

import type { RefObject } from "react";
import { Wrench } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";

import { getLocalizedToolTitle } from "../../../tool-activity";
import type {
  ProcessSummaryDetail,
  ProcessSummaryProjection,
} from "../../process/message-process-summary";
import type {
  AssistantActivityState,
  AssistantContentEnvironment,
  AssistantPermissionState,
  AssistantProcessState,
} from "./assistant-message-model";
import { AssistantToolRuns } from "./assistant-dm-tool-runs";
import { MessageDetailToggle } from "../../../ui/message-detail-toggle";


interface AssistantProcessCallchainProps {
  activity: AssistantActivityState;
  environment: AssistantContentEnvironment;
  permissions: AssistantPermissionState;
  process: AssistantProcessState;
}

export function AssistantProcessCallchain({
  activity,
  environment,
  permissions,
  process,
}: AssistantProcessCallchainProps) {
  if (!process.visible) {
    return null;
  }

  return (
    <div ref={process.anchorRef as RefObject<HTMLDivElement>}>
      <ProcessToggleButton process={process} />
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

function ProcessToggleButton({ process }: { process: AssistantProcessState }) {
  const { t } = useI18n();
  return (
    <MessageDetailToggle
      expanded={process.expanded}
      leading={(
        <Wrench
          className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)"
          strokeWidth={1.5}
        />
      )}
      onClick={process.toggle}
    >
      {formatProcessSummary(process.summary, t)}
    </MessageDetailToggle>
  );
}

function formatProcessSummary(
  summary: ProcessSummaryProjection,
  t: I18nContextValue["t"],
): string {
  if (summary.kind === "waiting_permission") {
    return t("message.process_waiting_permission");
  }
  // 只保留最近动作与异常数；两者都没有时退回通用入口文案。
  const parts = [
    summary.errorCount > 0
      ? t(summary.errorCount === 1 ? "message.process_error_one" : "message.process_error_other", { count: summary.errorCount })
      : null,
    summary.latestDetail ? formatProcessDetail(summary.latestDetail, t) : null,
  ].filter((part): part is string => part !== null);
  return parts.length > 0 ? parts.join(" · ") : t("message.process_view");
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
      <AssistantToolRuns
        activity={activity}
        environment={environment}
        permissions={permissions}
        projection={process.projection}
        responseResumed={false}
      />
    </div>
  );
}
