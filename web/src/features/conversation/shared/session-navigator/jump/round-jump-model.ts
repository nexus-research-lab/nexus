import type { ConversationTimeline } from "../../timeline/timeline-model";

export interface PendingRoundJump {
  navigationRoundId: string;
  scopeKey: string;
  scrollRoundId: string;
}

export function isRoundLoaded(
  timeline: ConversationTimeline,
  roundId: string,
): boolean {
  return (
    (timeline.message_groups.get(roundId)?.length ?? 0) > 0 ||
    timeline.live_round_ids.includes(roundId)
  );
}

export function isSameRoundJump(
  left: PendingRoundJump,
  right: PendingRoundJump,
): boolean {
  return (
    left.scopeKey === right.scopeKey &&
    left.navigationRoundId === right.navigationRoundId &&
    left.scrollRoundId === right.scrollRoundId
  );
}
