import { getErrorMessage } from "@/lib/error-message";

import type { FeedbackState } from "./provider-settings-types";

export function buildProviderErrorFeedback(
  error: unknown,
  title: string,
  fallbackMessage: string,
): FeedbackState {
  return {
    tone: "error",
    title,
    message: getErrorMessage(error, fallbackMessage),
  };
}
