import type { FeedbackState } from "./provider-settings-types";

export function getProviderErrorMessage(
  error: unknown,
  fallbackMessage: string,
): string {
  return error instanceof Error ? error.message : fallbackMessage;
}

export function buildProviderErrorFeedback(
  error: unknown,
  title: string,
  fallbackMessage: string,
): FeedbackState {
  return {
    tone: "error",
    title,
    message: getProviderErrorMessage(error, fallbackMessage),
  };
}
