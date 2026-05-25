export function safe_json_stringify(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

export function format_operation_time(value?: string | number | null): string {
  if (value == null || value === "") {
    return "just now";
  }
  const timestamp = typeof value === "number" ? value : Date.parse(value);
  if (!Number.isFinite(timestamp)) {
    return String(value);
  }
  const delta_seconds = Math.max(0, Math.floor((Date.now() - timestamp) / 1000));
  if (delta_seconds < 60) {
    return `${delta_seconds}s ago`;
  }
  const delta_minutes = Math.floor(delta_seconds / 60);
  if (delta_minutes < 60) {
    return `${delta_minutes}m ago`;
  }
  const delta_hours = Math.floor(delta_minutes / 60);
  if (delta_hours < 24) {
    return `${delta_hours}h ago`;
  }
  return new Date(timestamp).toLocaleString();
}
