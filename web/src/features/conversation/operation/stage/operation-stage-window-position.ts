import type { StageWindowState } from "../operation-desktop-types";
import type { StageNarrativePhase } from "./operation-stage-model";

const STAGE_MANAGER_BACKGROUND_POSITIONS = [
  "left-[3.5%] top-[18%] h-[13%] w-[10%]",
  "left-[3.5%] top-[34%] h-[13%] w-[10%]",
  "left-[3.5%] top-[50%] h-[13%] w-[10%]",
  "left-[3.5%] top-[66%] h-[13%] w-[10%]",
];

const PRIMARY_WORKSPACE = "left-[14%] top-[6%] h-[61%] w-[61%]";
const PRIMARY_REVIEW_WORKSPACE = "left-[14%] top-[6%] h-[60%] w-[59%]";
const COMPACT_REVIEW_WORKSPACE = "left-[22%] top-[9%] h-[54%] w-[58%]";

export function position_for_window(
  window: StageWindowState,
  narrative_phase: StageNarrativePhase,
  background_index = 0,
): string {
  const is_review_layout = narrative_phase === "completed";
  if (is_stage_manager_background_window(window, narrative_phase)) {
    return STAGE_MANAGER_BACKGROUND_POSITIONS[
      Math.min(background_index, STAGE_MANAGER_BACKGROUND_POSITIONS.length - 1)
    ];
  }
  if (window.layout === "terminal") {
    if (is_review_layout) {
      return window.phase === "focused"
        ? "left-[14%] top-[19%] h-[50%] w-[60%]"
        : "left-[15%] bottom-[18%] h-[18%] w-[60%]";
    }
    return window.phase === "focused"
      ? "left-[15%] top-[22%] h-[44%] w-[60%]"
      : "left-[14%] bottom-[18%] h-[18%] w-[58%]";
  }
  if (window.layout === "inspector") {
    return window.phase === "minimized"
      ? is_review_layout ? "right-[24%] bottom-[9%] h-14 w-[18%]" : "right-[6%] bottom-[9%] h-14 w-[18%]"
      : is_review_layout ? "right-[19%] bottom-[9%] h-[20%] w-[22%]" : "right-[4%] bottom-[9%] h-[21%] w-[22%]";
  }
  if (window.layout === "secondary") {
    return "left-[3%] top-[17%] h-[32%] w-[12%]";
  }
  if (window.kind === "permission_wait") {
    return window.phase === "minimized"
      ? "left-[36%] bottom-[8%] h-16 w-[28%]"
      : is_review_layout ? COMPACT_REVIEW_WORKSPACE : "left-[23%] top-[14%] h-[62%] w-[56%]";
  }
  if (window.layout === "artifact") {
    return window.phase === "minimized"
      ? is_review_layout ? "right-[22%] bottom-[9%] h-14 w-[20%]" : "right-[6%] bottom-[9%] h-14 w-[22%]"
      : is_review_layout ? "right-[9%] top-[15%] h-[52%] w-[32%]" : "right-[5%] top-[16%] h-[46%] w-[31%]";
  }
  if (window.kind === "browser") {
    return window.phase === "focused"
      ? is_review_layout ? PRIMARY_REVIEW_WORKSPACE : PRIMARY_WORKSPACE
      : is_review_layout ? "right-[14%] top-[15%] h-[42%] w-[32%]" : "right-[4%] top-[13%] h-[40%] w-[27%]";
  }
  if (window.kind === "code_editor") {
    return window.phase === "focused"
      ? is_review_layout ? PRIMARY_REVIEW_WORKSPACE : PRIMARY_WORKSPACE
      : is_review_layout ? "left-[17%] top-[16%] h-[44%] w-[30%]" : "left-[6%] top-[16%] h-[42%] w-[26%]";
  }
  if (window.kind === "task_board") {
    return is_review_layout ? COMPACT_REVIEW_WORKSPACE : PRIMARY_WORKSPACE;
  }
  if (window.kind === "generic_tool") {
    return window.phase === "focused"
      ? is_review_layout ? PRIMARY_REVIEW_WORKSPACE : PRIMARY_WORKSPACE
      : is_review_layout ? "left-[17%] top-[16%] h-[44%] w-[34%]" : "left-[16%] top-[15%] h-[45%] w-[36%]";
  }
  if (window.kind === "run_manifest") {
    return is_review_layout ? PRIMARY_REVIEW_WORKSPACE : PRIMARY_WORKSPACE;
  }
  if (window.kind === "handoff") {
    return window.phase === "focused"
      ? "left-[14%] top-[9%] h-[61%] w-[60%]"
      : "left-[23%] top-[14%] h-[48%] w-[48%]";
  }
  if (window.kind === "summary") {
    return is_review_layout ? COMPACT_REVIEW_WORKSPACE : "left-[28%] top-[15%] h-[52%] w-[46%]";
  }
  return is_review_layout ? PRIMARY_REVIEW_WORKSPACE : PRIMARY_WORKSPACE;
}

export function is_stage_manager_background_window(
  window: StageWindowState,
  _narrative_phase: StageNarrativePhase,
): boolean {
  return window.phase !== "focused"
    && window.phase !== "minimized";
}
