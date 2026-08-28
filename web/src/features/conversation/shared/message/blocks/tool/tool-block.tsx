"use client";

import type { RefObject } from "react";

import { ToolBlockHeader } from "./header/tool-block-header";
import { ToolBlockResult } from "./tool-block-detail";
import { ToolBlockPermission } from "./tool-block-permission";
import type {
  ToolBlockProps,
  ToolBlockViewModel,
  ToolPermissionRequest,
} from "./tool-block-types";
import { useToolBlockController } from "./use-tool-block-controller";
import { isSubagentToolName } from "../../message-tool-names";
import { SubagentTaskToolEntry } from "./subagent-task-tool-entry";
import { MessageDetailFrame } from "../../ui/message-rail";

export function ToolBlock({
  toolUse,
  toolResult,
  liveProgress,
  status = "success",
  startTime,
  endTime,
  permissionRequest,
  interactionDisabled = false,
  interactionDisabledReason,
  onOpenSubagentTask,
  onOpenWorkspaceFile,
  workspaceAgentId,
}: ToolBlockProps) {
  const controller = useToolBlockController({
    endTime,
    interactionDisabled,
    interactionDisabledReason,
    liveProgress,
    permissionRequest,
    startTime,
    status,
    toolResult,
    toolUse,
  });
  if (
    isSubagentToolName(toolUse.name)
    && onOpenSubagentTask
    && controller.model.status !== "waiting_permission"
  ) {
    return (
      <div
        className="message-cjk-font min-w-0"
        ref={controller.anchorRef as RefObject<HTMLDivElement>}
      >
        <SubagentTaskToolEntry
          model={controller.model}
          onOpen={() => onOpenSubagentTask(toolUse.id, workspaceAgentId)}
          toolUse={toolUse}
        />
      </div>
    );
  }
  return (
    <div
      className="message-cjk-font group/tool-block min-w-0"
      ref={controller.anchorRef as RefObject<HTMLDivElement>}
    >
      <ToolBlockHeader {...controller.header} />
      <OptionalToolDetails
        isExpanded={controller.isExpanded}
        model={controller.model}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        toolResult={toolResult}
        workspaceAgentId={workspaceAgentId}
      />
      <OptionalPermission
        interactionDisabled={interactionDisabled}
        interactionDisabledReason={interactionDisabledReason}
        model={controller.model}
        permissionRequest={permissionRequest}
        {...controller.permission}
      />
    </div>
  );
}

function OptionalToolDetails({
  isExpanded,
  model,
  onOpenWorkspaceFile,
  toolResult,
  workspaceAgentId,
}: Pick<
  ToolBlockProps,
  "onOpenWorkspaceFile" | "toolResult" | "workspaceAgentId"
> & { isExpanded: boolean; model: ToolBlockViewModel }) {
  if (!isExpanded) {
    return null;
  }
  const inputDetail = model.expandedDetailText !== model.collapsedDetailText
    ? model.expandedDetailText
    : null;
  if (!inputDetail && !toolResult) {
    return null;
  }
  return (
    <MessageDetailFrame>
      <div data-tool-block-details>
        {inputDetail ? (
          <pre
            className="message-cjk-font whitespace-pre-wrap break-all text-xs text-(--text-muted)"
            data-tool-block-input-detail
          >
            {inputDetail}
          </pre>
        ) : null}
        {toolResult ? (
          <div className={inputDetail ? "mt-1.5" : undefined}>
            <ToolBlockResult
              onOpenWorkspaceFile={onOpenWorkspaceFile}
              toolResult={toolResult}
              workspaceAgentId={workspaceAgentId}
            />
          </div>
        ) : null}
      </div>
    </MessageDetailFrame>
  );
}

function OptionalPermission({
  interactionDisabled,
  interactionDisabledReason,
  model,
  onSelectedSuggestionIndexChange,
  permissionRequest,
  selectedSuggestionIndex,
}: {
  interactionDisabled: boolean;
  interactionDisabledReason?: string;
  model: ToolBlockViewModel;
  onSelectedSuggestionIndexChange: (index: number) => void;
  permissionRequest?: ToolPermissionRequest;
  selectedSuggestionIndex: number;
}) {
  if (!permissionRequest || model.status !== "waiting_permission") {
    return null;
  }
  return (
    <ToolBlockPermission
      interactionDisabled={interactionDisabled}
      interactionDisabledReason={interactionDisabledReason}
      model={model}
      onSelectedSuggestionIndexChange={onSelectedSuggestionIndexChange}
      permissionRequest={permissionRequest}
      selectedSuggestionIndex={selectedSuggestionIndex}
    />
  );
}
