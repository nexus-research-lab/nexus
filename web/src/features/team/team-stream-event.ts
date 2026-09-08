import type {
  TeamBootstrap,
  TeamStreamResetRequired,
  TeamStreamUpdated,
} from "@/lib/api/conversation/team-api";

export function parseTeamStreamEvent(
  raw: unknown,
  bootstrap: TeamBootstrap,
): TeamStreamUpdated | TeamStreamResetRequired | null {
  if (!raw || typeof raw !== "object") {
    return null;
  }
  const update = raw as Partial<TeamStreamUpdated>;
  if (update.type === "stream.updated"
    && update.stream_id === bootstrap.conversation.sync_stream_id
    && update.stream_epoch === bootstrap.conversation.stream_epoch
    && typeof update.high_water_seq === "number"
    && Number.isSafeInteger(update.high_water_seq)
    && update.high_water_seq >= 0) {
    return update as TeamStreamUpdated;
  }
  const reset = raw as Partial<TeamStreamResetRequired>;
  return reset.type === "stream.reset_required"
    && reset.stream_id === bootstrap.conversation.sync_stream_id
    && reset.reason === "full_snapshot_required"
    ? reset as TeamStreamResetRequired
    : null;
}
