"use client";

export function is_provider_error(error: string): boolean {
  const normalized_error = error.toLowerCase();
  if (
    normalized_error.includes("provider_error=") ||
    normalized_error.includes("overloaded_error") ||
    normalized_error.includes("rate_limit_error")
  ) {
    return false;
  }
  return normalized_error.includes("provider");
}
