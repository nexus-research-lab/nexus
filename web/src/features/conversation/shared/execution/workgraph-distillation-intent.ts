/**
 * INPUT: WorkGraph Surface 中用户确认的源图、Slash 名称与节点角色。
 * OUTPUT: 当前 Session Composer 可消费的可见 Agent 请求。
 * POS: UI 到 execution-orchestrator Skill/CLI 沉淀链的意图桥；不写 Workflow API。
 */

export const WORKGRAPH_DISTILLATION_INTENT_EVENT =
  "nexus:workgraph-distillation-intent";
export const WORKGRAPH_WORKFLOWS_CHANGED_EVENT =
  "nexus:workgraph-workflows-changed";

export interface WorkGraphDistillationIntentDetail {
  prompt: string;
  sessionKey: string;
}

export function dispatchWorkGraphDistillationIntent(
  detail: WorkGraphDistillationIntentDetail,
) {
  window.dispatchEvent(new CustomEvent(WORKGRAPH_DISTILLATION_INTENT_EVENT, {
    detail,
  }));
}
