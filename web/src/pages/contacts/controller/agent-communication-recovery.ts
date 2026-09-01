/**
 * INPUT: 联系人写操作、exact UI intent 与服务端/传输失败。
 * OUTPUT: 保守的数据影响事实和是否阻止同一意图重复提交。
 * POS: Contacts 写失败纯投影；不执行刷新、重试或业务修改。
 */
import { projectMutationFailure } from "@/lib/error-message";
import type {
  AgentCommunicationMutationFailure,
  AgentCommunicationMutationKind,
} from "@/types/agent/communication";

export function buildAgentCommunicationMutationFailure(
  error: unknown,
  kind: AgentCommunicationMutationKind,
  intentKey: string,
  fallback: string,
  targetAgentId?: string,
): AgentCommunicationMutationFailure {
  const projected = projectMutationFailure(error, fallback);
  return {
    blocksRepeat: projected.effect !== "not_applied",
    effect: projected.effect,
    intentKey,
    kind,
    message: projected.message,
    ...(targetAgentId ? { targetAgentId } : {}),
  };
}

export function blocksAgentCommunicationIntent(
  failure: AgentCommunicationMutationFailure | null,
  kind: AgentCommunicationMutationKind,
  intentKey: string,
): boolean {
  return Boolean(
    failure?.blocksRepeat
    && failure.kind === kind
    && failure.intentKey === intentKey,
  );
}

export function reconcileContactDirectoryMutation(
  failure: AgentCommunicationMutationFailure | null,
  contactAgentIds: ReadonlySet<string>,
): AgentCommunicationMutationFailure | null {
  if (!failure?.targetAgentId
    || (failure.kind !== "add_contact" && failure.kind !== "remove_contact")) {
    return failure;
  }

  const contactExists = contactAgentIds.has(failure.targetAgentId);
  const intendedStateExists = failure.kind === "add_contact";
  if (contactExists === intendedStateExists) {
    return null;
  }
  return {
    ...failure,
    blocksRepeat: false,
    effect: "not_applied",
  };
}
