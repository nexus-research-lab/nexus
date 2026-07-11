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
  return (
    <div
      className="message-cjk-font group/tool-block min-w-0"
      ref={controller.anchorRef as RefObject<HTMLDivElement>}
    >
      <ToolBlockHeader {...controller.header} />
      <RunningProgress model={controller.model} />
      <OptionalToolResult
        isExpanded={controller.isExpanded}
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

function RunningProgress({ model }: { model: ToolBlockViewModel }) {
  if (model.hasResult || model.status !== "running") {
    return null;
  }
  return (
    <div className="ml-7 mt-1 h-px overflow-hidden rounded-full bg-primary/15">
      <div className="h-full w-1/3 animate-pulse rounded-full bg-primary/60" />
    </div>
  );
}

function OptionalToolResult({
  isExpanded,
  onOpenWorkspaceFile,
  toolResult,
  workspaceAgentId,
}: Pick<
  ToolBlockProps,
  "onOpenWorkspaceFile" | "toolResult" | "workspaceAgentId"
> & { isExpanded: boolean }) {
  if (!toolResult || !isExpanded) {
    return null;
  }
  return (
    <ToolBlockResult
      onOpenWorkspaceFile={onOpenWorkspaceFile}
      toolResult={toolResult}
      workspaceAgentId={workspaceAgentId}
    />
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
