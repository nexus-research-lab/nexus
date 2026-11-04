// INPUT: 当前 Session 的消息终态、队列、共享 WorkGraph 恢复 owner 与输入可用性。
// OUTPUT: 发送位置的继续动作；同一意图防重、未知受理不重发、跨会话迟到结果不写回。
// POS: DM/Room 统一继续编排；不改写历史，也不在浏览器创造 Attempt。
import { useEffect, useRef } from "react";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useSharedExecutionContinuation } from "../../execution/execution-continuation-state";
import type { ComposerPanelProps } from "../composer-model";
import { resolveConversationContinuation } from "./composer-continuation-model";
import { shouldRestoreMessageDraftAfterFailure } from "./use-composer-message-submit";

export function useComposerContinuation({
  scope, sessionKey, messages, ready, memberIds, inputQueueItems, onSendMessage, onGuideQueuedMessage,
}: {
  scope: string;
  sessionKey?: string;
  messages: NonNullable<ComposerPanelProps["continuationMessages"]>;
  ready: boolean;
  memberIds?: readonly string[];
  inputQueueItems: ComposerPanelProps["inputQueueItems"];
  onSendMessage: ComposerPanelProps["onSendMessage"];
  onGuideQueuedMessage: ComposerPanelProps["onGuideQueuedMessage"];
}) {
  const { t } = useI18n();
  const shared = useSharedExecutionContinuation();
  const owner = useRef({ scope });
  if (owner.current.scope !== scope) owner.current = { scope };
  useEffect(() => {
    if (owner.current.scope !== scope) owner.current = { scope };
    const active = owner.current;
    return () => { if (owner.current === active) owner.current = { scope: "" }; };
  }, [scope]);
  const graph = shared?.resource.sessionKey === sessionKey ? shared : null;
  const nodes = graph?.resource.execution?.graph?.nodes?.filter((node) => node.can_recover) ?? [];
  const candidate = resolveConversationContinuation(messages, memberIds);
  const managed = candidate && graph?.resource.execution?.graph?.nodes?.some((node) => node.work_item_id && node.agent_round_id && candidate.agentRoundIds.includes(node.agent_round_id));
  const ordinary = managed ? null : candidate;
  const queued = inputQueueItems.find((item) => item.dispatch_paused);
  const key = nodes.length ? `graph:${graph?.resource.execution?.id}:${nodes.map((node) => node.attempt_id).sort().join(":")}`
    : queued ? `queue:${queued.id}` : ordinary ? `message:${ordinary.key}` : "";
  const identity = `${scope}\u0000${key}`;
  const [status, setStatus] = useResettableState<"idle" | "pending" | "accepted" | "unknown" | "failed">("idle", identity);
  const current = useRef({ scope, identity, busy: false });
  if (current.current.identity !== identity) current.current = { scope, identity, busy: false };
  useEffect(() => {
    if (current.current.scope !== scope) current.current = { scope, identity, busy: false };
    const active = current.current;
    return () => { if (current.current === active) current.current = { scope: "", identity: "", busy: false }; };
  }, [scope, identity]);
  const graphBlocked = Boolean(graph && (graph.resource.isLoading || graph.resource.isStale));
  const graphStates = nodes.map((node) => graph?.action.states[node.attempt_id ?? ""]?.status);
  const checking = graphStates.includes("unknown");
  const waiting = graphStates.some((state) => state === "pending" || state === "queued");
  const disabled = !ready || (!nodes.length && !queued && inputQueueItems.length > 0) || graphBlocked || waiting || status === "pending" || status === "accepted" || status === "unknown";
  const run = async () => {
    if (!key || disabled || current.current.identity !== identity || current.current.busy) return;
    const active = current.current;
    const activeOwner = owner.current;
    active.busy = true;
    setStatus("pending");
    try {
      if (nodes.length && graph) {
        // 串行提交：每次服务端重新读取 Execution revision，避免同批节点互相制造版本冲突。
        for (const node of nodes) {
          if (owner.current !== activeOwner) break;
          await graph.action.run(node);
        }
        if (current.current === active) setStatus("idle");
      } else {
        if (queued) await onGuideQueuedMessage(queued.id);
        else await onSendMessage(t("composer.continue_message"), "queue", [], ordinary?.targetAgentIds ?? []);
        if (current.current === active) setStatus("accepted");
      }
    } catch (error) {
      if (current.current === active) setStatus(shouldRestoreMessageDraftAfterFailure(error) ? "failed" : "unknown");
    } finally {
      if (current.current === active) active.busy = false;
    }
  };
  return {
    available: Boolean(key), disabled, run,
    label: t(checking ? "execution.recovery_check" : status === "unknown" ? "composer.continue_unknown" : waiting || status === "pending" || status === "accepted" ? "composer.continue_pending" : "composer.continue"),
    error: status === "failed" ? t("composer.continue_failed") : status === "unknown" ? t("composer.continue_unknown_impact") : null,
  };
}
