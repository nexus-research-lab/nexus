export type UnknownRecord = Record<string, unknown>;

export function asUnknownRecord(value: unknown): UnknownRecord | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? value as UnknownRecord
    : null;
}

export function readString(
  record: UnknownRecord,
  key: string,
): string | null {
  const value = record[key];
  return typeof value === "string" ? value : null;
}

export function readNumber(
  record: UnknownRecord,
  key: string,
): number | null {
  const value = record[key];
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

export function readBoolean(
  record: UnknownRecord,
  key: string,
): boolean | null {
  const value = record[key];
  return typeof value === "boolean" ? value : null;
}

export function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => typeof item === "string");
}
