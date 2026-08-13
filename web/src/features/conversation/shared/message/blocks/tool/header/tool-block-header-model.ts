import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";

import type {
  ToolBlockStatus,
  ToolBlockViewModel,
} from "../tool-block-types";

type ExpansionState = "collapsed" | "expanded";

export interface ToolBlockHeaderProjection {
  canCopyResult: boolean;
  canToggle: boolean;
  detailClassName: string;
  detailText: string;
  expansionState: ExpansionState;
  liveStatusText: string | null;
  metaText: string | null;
  showPermissionActions: boolean;
  stateClassName: string;
}

const EXPANSION_STATE_BY_FLAG: Readonly<Record<string, ExpansionState>> = {
  false: "collapsed",
  true: "expanded",
};

const DETAIL_CLASS_BY_EXPANSION: Readonly<Record<ExpansionState, string>> = {
  collapsed: "truncate",
  expanded: "whitespace-pre-wrap break-all",
};

const HEADER_STATE_CLASS_BY_STATUS: Readonly<Record<ToolBlockStatus, string>> = {
  error: "",
  pending: "",
  running: "bg-primary/5",
  rejected: "",
  superseded: "bg-(--surface-muted-background)",
  stopped: "bg-(--surface-muted-background)",
  success: "",
  waiting_permission: "bg-(--surface-muted-background)",
};

const DETAIL_FALLBACK_KEY_BY_STATUS: Readonly<
  Record<ToolBlockStatus, TranslationKey>
> = {
  error: "message.tool_processing",
  pending: "message.tool_processing",
  running: "message.tool_processing",
  rejected: "message.tool_rejection_without_detail",
  superseded: "message.tool_superseded_without_detail",
  stopped: "message.stopped",
  success: "message.tool_processing",
  waiting_permission: "message.tool_waiting_confirmation",
};

const META_TEXT_BY_STATUS: Readonly<Record<
  ToolBlockStatus,
  (model: ToolBlockViewModel) => string | null
>> = {
  error: (model) => model.durationText,
  pending: (model) => model.durationText,
  running: (model) => model.durationText,
  rejected: (model) => model.durationText,
  superseded: (model) => model.durationText,
  stopped: (model) => model.durationText,
  success: (model) => model.durationText,
  waiting_permission: (model) => model.waitingActionHint,
};

const LIVE_STATUS_BY_STATUS: Readonly<Record<
  ToolBlockStatus,
  (model: ToolBlockViewModel) => string | null
>> = {
  error: () => null,
  pending: () => null,
  running: (model) => model.liveStatusText,
  rejected: () => null,
  superseded: () => null,
  stopped: () => null,
  success: () => null,
  waiting_permission: () => null,
};

export function buildToolBlockHeaderProjection(
  model: ToolBlockViewModel,
  isExpanded: boolean,
  t: I18nContextValue["t"],
): ToolBlockHeaderProjection {
  const expansionState = EXPANSION_STATE_BY_FLAG[String(isExpanded)];
  const detailByExpansion: Readonly<Record<ExpansionState, string | null>> = {
    collapsed: model.collapsedDetailText,
    expanded: model.expandedDetailText,
  };
  return {
    canCopyResult: [
      model.hasResult,
      model.status !== "waiting_permission",
    ].every(Boolean),
    canToggle: model.hasResult,
    detailClassName: DETAIL_CLASS_BY_EXPANSION[expansionState],
    detailText: firstText([
      detailByExpansion[expansionState],
      t(DETAIL_FALLBACK_KEY_BY_STATUS[model.status]),
    ]),
    expansionState,
    liveStatusText: LIVE_STATUS_BY_STATUS[model.status](model),
    metaText: META_TEXT_BY_STATUS[model.status](model),
    showPermissionActions: model.status === "waiting_permission",
    stateClassName: HEADER_STATE_CLASS_BY_STATUS[model.status],
  };
}

function firstText(candidates: Array<string | null>): string {
  return candidates.find(Boolean) ?? "";
}
