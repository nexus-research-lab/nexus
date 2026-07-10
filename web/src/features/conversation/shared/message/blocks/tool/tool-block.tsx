"use client";

import { useCallback } from "react";

import { useScrollAnchoredState } from "@/hooks/conversation/use-scroll-anchored-state";
import { useCopyToClipboard } from "@/hooks/ui/use-copy-to-clipboard";
import { useResettableState } from "@/hooks/ui/use-resettable-state";

import { ToolBlockHeader } from "./tool-block-header";
import { buildToolBlockViewModel } from "./tool-block-model";
import { ToolBlockPermission } from "./tool-block-permission";
import { ToolBlockResult } from "./tool-block-result";
import type { ToolBlockProps } from "./tool-block-types";

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
  const {
    isOpen: isExpanded,
    toggle: toggleExpanded,
    anchorRef: toolAnchorRef,
  } = useScrollAnchoredState(false);
  const [selectedSuggestionIndex, setSelectedSuggestionIndex] =
    useResettableState(-1, permissionRequest?.request_id ?? null);
  const { copied, copy } = useCopyToClipboard();
  const model = buildToolBlockViewModel({
    toolUse,
    toolResult,
    liveProgress,
    status,
    startTime,
    endTime,
    permissionRequest,
    interactionDisabled,
    interactionDisabledReason,
  });

  const handleCopyResult = useCallback(async () => {
    if (!toolResult) return;
    const content = typeof toolResult.content === "string"
      ? toolResult.content
      : JSON.stringify(toolResult.content, null, 2);
    await copy(content);
  }, [copy, toolResult]);
  const handleAllow = useCallback(() => {
    if (!permissionRequest) return;
    const selectedUpdate = selectedSuggestionIndex >= 0
      ? permissionRequest.suggestions?.[selectedSuggestionIndex]
      : undefined;
    permissionRequest.on_allow(selectedUpdate ? [selectedUpdate] : undefined);
  }, [permissionRequest, selectedSuggestionIndex]);
  const handleDeny = useCallback(() => {
    permissionRequest?.on_deny();
  }, [permissionRequest]);

  return (
    <div
      ref={toolAnchorRef as React.RefObject<HTMLDivElement>}
      className="message-cjk-font group/tool-block min-w-0"
    >
      <ToolBlockHeader
        copied={copied}
        interactionDisabled={interactionDisabled}
        interactionDisabledReason={interactionDisabledReason}
        isExpanded={isExpanded}
        model={model}
        onAllow={permissionRequest ? handleAllow : undefined}
        onCopyResult={handleCopyResult}
        onDeny={permissionRequest ? handleDeny : undefined}
        onToggle={toggleExpanded}
      />

      {!model.hasResult && model.isRunning ? (
        <div className="ml-7 mt-1 h-px overflow-hidden rounded-full bg-primary/15">
          <div className="h-full w-1/3 animate-pulse rounded-full bg-primary/60" />
        </div>
      ) : null}

      {toolResult && isExpanded ? (
        <ToolBlockResult
          toolResult={toolResult}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          workspaceAgentId={workspaceAgentId}
        />
      ) : null}

      {permissionRequest && model.isWaiting ? (
        <ToolBlockPermission
          interactionDisabled={interactionDisabled}
          interactionDisabledReason={interactionDisabledReason}
          model={model}
          onSelectedSuggestionIndexChange={setSelectedSuggestionIndex}
          permissionRequest={permissionRequest}
          selectedSuggestionIndex={selectedSuggestionIndex}
        />
      ) : null}
    </div>
  );
}
