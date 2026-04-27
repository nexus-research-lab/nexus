"use client";

export function is_provider_error(error: string): boolean {
  return error.toLowerCase().includes("provider");
}
