"use client";

import { useCallback, useRef, useState } from "react";

import { write_text_to_clipboard } from "./clipboard";

const COPY_FEEDBACK_TIMEOUT_MS = 2000;

export interface UseCopyToClipboardOptions {
  feedback_timeout_ms?: number;
}

export interface UseCopyToClipboardResult {
  copied: boolean;
  copy: (text: string) => Promise<boolean>;
}

export function useCopyToClipboard(
  options: UseCopyToClipboardOptions = {},
): UseCopyToClipboardResult {
  const timeout_ms = options.feedback_timeout_ms ?? COPY_FEEDBACK_TIMEOUT_MS;
  const [copied, set_copied] = useState(false);
  const reset_timer_ref = useRef<ReturnType<typeof setTimeout> | null>(null);

  const copy = useCallback(
    async (text: string): Promise<boolean> => {
      if (!text) return false;
      const succeeded = await write_text_to_clipboard(text);
      if (succeeded) {
        set_copied(true);
        if (reset_timer_ref.current) {
          clearTimeout(reset_timer_ref.current);
        }
        reset_timer_ref.current = setTimeout(() => {
          set_copied(false);
          reset_timer_ref.current = null;
        }, timeout_ms);
        return true;
      }
      console.error("[useCopyToClipboard] copy failed");
      return false;
    },
    [timeout_ms],
  );

  return { copied, copy };
}
