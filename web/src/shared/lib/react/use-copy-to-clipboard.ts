// INPUT: 显式复制动作及本地成功反馈保留时长。
// OUTPUT: 剪贴板写入结果和短暂 copied 状态；卸载释放计时器并忽略迟到反馈。
// POS: 中立 React 反馈生命周期；复制权限与原生回退归 browser/clipboard。
"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { writeTextToClipboard } from "../browser/clipboard";

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
  const isMountedRef = useRef(false);

  useEffect(() => {
    isMountedRef.current = true;
    return () => {
      isMountedRef.current = false;
      if (resetTimerRef.current) {
        clearTimeout(resetTimerRef.current);
        resetTimerRef.current = null;
      }
    };
  }, []);

  const copy = useCallback(
    async (text: string): Promise<boolean> => {
      if (!text) return false;
      const succeeded = await writeTextToClipboard(text);
      if (succeeded) {
        if (!isMountedRef.current) return true;
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
