export function formatElapsed(
  started_at: number | undefined,
  ended_at: number | null | undefined,
  updated_at: number,
): string {
  const start = normalize_timestamp(started_at ?? updated_at);
  const end = normalize_timestamp(ended_at ?? updated_at);
  const seconds = Math.max(0, Math.round((end - start) / 1000));
  if (seconds < 60) {
    return `${seconds}s`;
  }
  const minutes = Math.floor(seconds / 60);
  const remaining_seconds = seconds % 60;
  return `${minutes}m ${remaining_seconds}s`;
}

function normalize_timestamp(timestamp: number): number {
  return timestamp < 1_000_000_000_000 ? timestamp * 1000 : timestamp;
}
