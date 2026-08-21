/**
 * INPUT: 当前 feed/消息/运行态计数与浏览器内实际驻留的 round identities。
 * OUTPUT: 任一等量窗口替换也会变化的可见历史加载 revision。
 * POS: Session 时间线到 window-loader 内容失效协议的纯投影边界。
 */
interface VisibleRoundRevisionInput {
  feedRoundCount: number;
  liveRoundCount: number;
  loadedRoundIds: readonly string[];
  messageCount: number;
  pendingAgentSlotCount: number;
  pendingPermissionCount: number;
  roomAgentExecutionStateCount: number;
}

export function buildVisibleRoundRevision({
  feedRoundCount,
  liveRoundCount,
  loadedRoundIds,
  messageCount,
  pendingAgentSlotCount,
  pendingPermissionCount,
  roomAgentExecutionStateCount,
}: VisibleRoundRevisionInput): string {
  return [
    feedRoundCount,
    messageCount,
    pendingAgentSlotCount,
    pendingPermissionCount,
    roomAgentExecutionStateCount,
    liveRoundCount,
    loadedRoundIds.join("\u001e"),
  ].join("\u001f");
}
