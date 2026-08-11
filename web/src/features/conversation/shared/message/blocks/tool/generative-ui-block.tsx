/**
 * INPUT: show_widget tool_use 与对应 tool_result 完成状态。
 * OUTPUT: 流式更新、完成后执行脚本的隔离 iframe。
 * POS: 对话内生成式 UI 视图；只接受 iframe 自身的高度消息。
 */
"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Sparkles } from "lucide-react";

import { useTheme } from "@/shared/theme/theme-context";
import type { ToolUseContent } from "@/types/conversation/message/content";

import {
  buildGenerativeUIFinalDocument,
  buildGenerativeUIShellDocument,
  GENERATIVE_UI_MESSAGE_SOURCE,
  GENERATIVE_UI_RESIZE_MESSAGE,
  GENERATIVE_UI_UPDATE_MESSAGE,
} from "./generative-ui-document";

const UPDATE_DELAY_MS = 150;
const INITIAL_HEIGHT = 320;
const MIN_HEIGHT = 180;
const MAX_HEIGHT = 4000;

export function GenerativeUIBlock({
  complete,
  toolUse,
}: {
  complete: boolean;
  toolUse: ToolUseContent;
}) {
  const { theme } = useTheme();
  const frameRef = useRef<HTMLIFrameElement>(null);
  const [height, setHeight] = useState(INITIAL_HEIGHT);
  const input = toolUse.input ?? {};
  const title = typeof input.title === "string" ? input.title.trim() : "";
  const widgetCode = typeof input.widget_code === "string"
    ? input.widget_code
    : "";
  const dark = theme === "dark" || theme === "rain";
  const shellDocument = useMemo(
    () => buildGenerativeUIShellDocument(dark),
    [dark],
  );
  const finalDocument = useMemo(
    () => buildGenerativeUIFinalDocument(widgetCode, dark),
    [dark, widgetCode],
  );

  const sendStreamingUpdate = useCallback(() => {
    if (complete || !widgetCode) {
      return;
    }
    frameRef.current?.contentWindow?.postMessage({
      type: GENERATIVE_UI_UPDATE_MESSAGE,
      html: widgetCode,
    }, "*");
  }, [complete, widgetCode]);

  useEffect(() => {
    const timer = window.setTimeout(sendStreamingUpdate, UPDATE_DELAY_MS);
    return () => window.clearTimeout(timer);
  }, [sendStreamingUpdate]);

  useEffect(() => {
    const handleMessage = (event: MessageEvent) => {
      if (event.source !== frameRef.current?.contentWindow) {
        return;
      }
      const data = event.data as Record<string, unknown> | null;
      if (
        !data
        || data.source !== GENERATIVE_UI_MESSAGE_SOURCE
        || data.type !== GENERATIVE_UI_RESIZE_MESSAGE
        || typeof data.height !== "number"
        || !Number.isFinite(data.height)
      ) {
        return;
      }
      setHeight(Math.ceil(Math.min(MAX_HEIGHT, Math.max(MIN_HEIGHT, data.height))));
    };
    window.addEventListener("message", handleMessage);
    return () => window.removeEventListener("message", handleMessage);
  }, []);

  return (
    <section
      aria-busy={!complete}
      className="min-w-0 overflow-hidden rounded-2xl border border-(--surface-card-border) bg-(--surface-paper-background)"
      data-generative-ui="true"
    >
      <header className="flex h-10 items-center gap-2 border-b border-(--content-divider-color) px-3">
        <span className="flex h-6 w-6 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <Sparkles aria-hidden="true" className="h-3.5 w-3.5" />
        </span>
        <span className="min-w-0 flex-1 truncate text-xs font-medium text-(--text-strong)">
          {title || toolUse.name}
        </span>
        {!complete ? (
          <span
            aria-hidden="true"
            className="h-1.5 w-1.5 rounded-full bg-primary motion-safe:animate-pulse"
          />
        ) : null}
      </header>
      {widgetCode ? (
        <iframe
          className="block w-full border-0 bg-(--surface-paper-background)"
          loading="lazy"
          onLoad={sendStreamingUpdate}
          ref={frameRef}
          sandbox="allow-scripts"
          srcDoc={complete ? finalDocument : shellDocument}
          style={{ height }}
          title={title || toolUse.name}
        />
      ) : (
        <div className="h-[180px] bg-(--surface-panel-subtle-background) motion-safe:animate-pulse" />
      )}
    </section>
  );
}
