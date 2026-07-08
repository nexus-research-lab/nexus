"use client";

import { useCallback, useRef, useState } from "react";

import { writeTextToClipboard } from "./clipboard";

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
  const timeoutMs = options.feedback_timeout_ms ?? COPY_FEEDBACK_TIMEOUT_MS;
  const [copied, setCopied] = useState(false);
  const resetTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const copy = useCallback(
    async (text: string): Promise<boolean> => {
      if (!text) return false;
      const succeeded = await writeTextToClipboard(text);
      if (succeeded) {
        setCopied(true);
        if (resetTimerRef.current) {
          clearTimeout(resetTimerRef.current);
        }
        resetTimerRef.current = setTimeout(() => {
          setCopied(false);
          resetTimerRef.current = null;
        }, timeoutMs);
        return true;
      }
      console.error("[useCopyToClipboard] copy failed");
      return false;
    },
    [timeoutMs],
  );

  return { copied, copy };
}
