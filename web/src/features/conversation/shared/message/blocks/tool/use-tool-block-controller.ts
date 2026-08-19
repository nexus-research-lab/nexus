"use client";

/**
 * INPUT: Tool 生命周期、权限和可选 generation-bound 完整结果引用。
 * OUTPUT: 展开/复制/权限动作及稳定 ToolBlock 展示模型。
 * POS: ToolBlock 的交互与按需完整结果控制器。
 */

import { useCallback } from "react";

import { useScrollAnchoredState } from "@/features/conversation/shared/timeline/scroll/use-scroll-anchored-state";
import { useCopyToClipboard } from "@/hooks/ui/use-copy-to-clipboard";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getSessionMessageDetailApi } from "@/lib/api/conversation/session-api";
import type { PermissionUpdate } from "@/types/conversation/interaction/permission";
import type { ToolResultContent } from "@/types/conversation/message/content";

import { buildToolBlockViewModel } from "./tool-block-model";
import type { ToolBlockProps } from "./tool-block-types";

export function useToolBlockController({
  endTime,
  interactionDisabled = false,
  interactionDisabledReason,
  liveProgress,
  permissionRequest,
  startTime,
  status = "success",
  toolResult,
  toolUse,
}: Pick<
  ToolBlockProps,
  | "endTime"
  | "interactionDisabled"
  | "interactionDisabledReason"
  | "liveProgress"
  | "permissionRequest"
  | "startTime"
  | "status"
  | "toolResult"
  | "toolUse"
>) {
  const localization = useI18n();
  const expansion = useScrollAnchoredState(false);
  const [selectedSuggestionIndex, setSelectedSuggestionIndex] =
    useResettableState(-1, permissionRequest?.request_id ?? null);
  const { copied, copy } = useCopyToClipboard();
  const model = buildToolBlockViewModel({
    endTime,
    interactionDisabled,
    interactionDisabledReason,
    localization,
    liveProgress,
    permissionRequest,
    startTime,
    status,
    toolResult,
    toolUse,
  });
  const copyResult = useCallback(async () => {
    try {
      const content = getToolResultCopyText(
        await resolveCompleteToolResult(toolResult),
      );
      if (content === null) {
        return;
      }
      await copy(content);
    } catch (error) {
      console.error("[conversation] Failed to copy complete tool result", error);
    }
  }, [copy, toolResult]);
  const allow = useCallback(() => {
    if (!permissionRequest) {
      return;
    }
    permissionRequest.on_allow(getSelectedPermissionUpdates(
      permissionRequest.suggestions,
      selectedSuggestionIndex,
    ));
  }, [permissionRequest, selectedSuggestionIndex]);
  const deny = useCallback(() => {
    permissionRequest?.on_deny();
  }, [permissionRequest]);
  const permissionActions = projectPermissionActions(permissionRequest, allow, deny);

  return {
    anchorRef: expansion.anchorRef,
    header: {
      copied,
      interactionDisabled,
      interactionDisabledReason,
      isExpanded: expansion.isOpen,
      model,
      onAllow: permissionActions.onAllow,
      onCopyResult: copyResult,
      onDeny: permissionActions.onDeny,
      onToggle: expansion.toggle,
    },
    isExpanded: expansion.isOpen,
    model,
    permission: {
      onSelectedSuggestionIndexChange: setSelectedSuggestionIndex,
      selectedSuggestionIndex,
    },
  };
}

async function resolveCompleteToolResult(
  toolResult: ToolResultContent | undefined,
): Promise<ToolResultContent | undefined> {
  const detailRef = toolResult?.detail_ref?.trim();
  const sessionKey = toolResult?.detail_session_key?.trim();
  if (!toolResult || !detailRef || !sessionKey) {
    return toolResult;
  }
  const detail = await getSessionMessageDetailApi(sessionKey, detailRef);
  return { ...toolResult, content: detail.content };
}

function getToolResultCopyText(
  toolResult: ToolResultContent | undefined,
): string | null {
  if (!toolResult) {
    return null;
  }
  if (typeof toolResult.content === "string") {
    return toolResult.content;
  }
  return JSON.stringify(toolResult.content, null, 2);
}

function getSelectedPermissionUpdates(
  suggestions: PermissionUpdate[] | undefined,
  selectedIndex: number,
): PermissionUpdate[] | undefined {
  const selected = suggestions?.[selectedIndex];
  return selected ? [selected] : undefined;
}

function projectPermissionActions(
  permissionRequest: ToolBlockProps["permissionRequest"],
  onAllow: () => void,
  onDeny: () => void,
) {
  const rules = [
    {
      matches: Boolean(permissionRequest),
      value: { onAllow, onDeny },
    },
    {
      matches: true,
      value: { onAllow: undefined, onDeny: undefined },
    },
  ];
  return rules.find((rule) => rule.matches)!.value;
}
