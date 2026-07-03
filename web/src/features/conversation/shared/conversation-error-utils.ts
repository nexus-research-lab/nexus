"use client";

export function isProviderError(error: string): boolean {
  const normalizedError = error.toLowerCase();
  if (
    normalizedError.includes("provider_error=") ||
    normalizedError.includes("overloaded_error") ||
    normalizedError.includes("rate_limit_error")
  ) {
    return false;
  }
  return normalizedError.includes("provider");
}
