/**
 * INPUT: show_widget tool_use 与对应 tool_result 完成状态。
 * OUTPUT: 流式更新、完成后执行脚本，并以 Problem / Impact / Recovery 展示缺失或运行失败的隔离 iframe。
 * POS: 对话内生成式 UI 视图；失败只影响本地内容块，只接受 iframe 自身的高度与运行状态消息。
 */
"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { RotateCcw } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { useTheme } from "@/shared/theme/theme-context";
import { cn } from "@/shared/ui/class-name";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UiSkeleton } from "@/shared/ui/display/skeleton";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { ToolUseContent } from "@/types/conversation/message/content";

import {
  buildGenerativeUIShellDocument,
  GENERATIVE_UI_ERROR_MESSAGE,
  GENERATIVE_UI_MESSAGE_SOURCE,
  GENERATIVE_UI_READY_MESSAGE,
  GENERATIVE_UI_RESIZE_MESSAGE,
  GENERATIVE_UI_UPDATE_MESSAGE,
} from "./generative-ui-document";
import { resolveGenerativeUIHeightRevision } from "./generative-ui-height-model";

const UPDATE_DELAY_MS = 150;
const FINAL_HEIGHT_SETTLE_MS = 80;
const INITIAL_HEIGHT = 320;
const MAX_ERROR_MESSAGE_LENGTH = 240;

type RenderState =
  | { status: "error"; message: string }
  | { status: "loading" | "ready" };

export function GenerativeUIBlock({
  complete,
  toolUse,
}: {
  complete: boolean;
  toolUse: ToolUseContent;
}) {
  const { t } = useI18n();
  const { theme } = useTheme();
  const frameRef = useRef<HTMLIFrameElement>(null);
  const [height, setHeight] = useState(INITIAL_HEIGHT);
  const heightRef = useRef(INITIAL_HEIGHT);
  const completeRef = useRef(complete);
  const heightSettleTimerRef = useRef<number | null>(null);
  const [renderState, setRenderState] = useState<RenderState>({
    status: "loading",
  });
  const input = toolUse.input ?? {};
  const title = typeof input.title === "string" ? input.title.trim() : "";
  const widgetCode = typeof input.widget_code === "string"
    ? input.widget_code
    : "";
  const visualTheme = theme === "sunny" ? "light" : theme;
  completeRef.current = complete;
  const shellDocument = useMemo(
    () => buildGenerativeUIShellDocument(visualTheme),
    [visualTheme],
  );

  const sendWidgetUpdate = useCallback(() => {
    if (!widgetCode) {
      return;
    }
    setRenderState({ status: "loading" });
    frameRef.current?.contentWindow?.postMessage({
      type: GENERATIVE_UI_UPDATE_MESSAGE,
      final: complete,
      html: widgetCode,
    }, "*");
  }, [complete, widgetCode]);

  const cancelHeightSettle = useCallback(() => {
    if (heightSettleTimerRef.current !== null) {
      window.clearTimeout(heightSettleTimerRef.current);
      heightSettleTimerRef.current = null;
    }
  }, []);

  const applyReportedHeight = useCallback((reportedHeight: number) => {
    const revision = resolveGenerativeUIHeightRevision(
      heightRef.current,
      reportedHeight,
      completeRef.current,
    );
    cancelHeightSettle();
    if (!revision.settle) {
      heightRef.current = revision.height;
      setHeight(revision.height);
      return;
    }
    heightSettleTimerRef.current = window.setTimeout(() => {
      heightSettleTimerRef.current = null;
      heightRef.current = revision.height;
      setHeight(revision.height);
    }, FINAL_HEIGHT_SETTLE_MS);
  }, [cancelHeightSettle]);

  useEffect(() => {
    if (complete) {
      sendWidgetUpdate();
      return;
    }
    const timer = window.setTimeout(sendWidgetUpdate, UPDATE_DELAY_MS);
    return () => window.clearTimeout(timer);
  }, [complete, sendWidgetUpdate]);

  useEffect(() => {
    cancelHeightSettle();
  }, [cancelHeightSettle, complete, widgetCode]);

  useEffect(() => cancelHeightSettle, [cancelHeightSettle]);

  useEffect(() => {
    const handleMessage = (event: MessageEvent) => {
      if (event.source !== frameRef.current?.contentWindow) {
        return;
      }
      const data = event.data as Record<string, unknown> | null;
      if (!data || data.source !== GENERATIVE_UI_MESSAGE_SOURCE) {
        return;
      }
      if (
        data.type === GENERATIVE_UI_RESIZE_MESSAGE
        && typeof data.height === "number"
        && Number.isFinite(data.height)
      ) {
        applyReportedHeight(data.height);
        return;
      }
      if (data.type === GENERATIVE_UI_READY_MESSAGE) {
        setRenderState({ status: "ready" });
        return;
      }
      if (data.type === GENERATIVE_UI_ERROR_MESSAGE) {
        const message = typeof data.message === "string" && data.message.trim()
          ? data.message.trim().slice(0, MAX_ERROR_MESSAGE_LENGTH)
          : t("generative_ui.render_failed_detail");
        setRenderState({ status: "error", message });
      }
    };
    window.addEventListener("message", handleMessage);
    return () => window.removeEventListener("message", handleMessage);
  }, [applyReportedHeight, t]);

  const missingWidgetCode = complete && !widgetCode;
  const visibleStatus = missingWidgetCode ? "error" : renderState.status;
  const loading = !complete || (!missingWidgetCode && renderState.status === "loading");

  return (
    <section
      aria-busy={loading}
      className="my-3 min-w-0 overflow-hidden surface-radius-sm bg-transparent"
      data-generative-ui="true"
      data-generative-ui-status={visibleStatus}
    >
      <header className="flex min-h-9 items-center gap-2 bg-(--surface-panel-background) px-3 py-2">
        <span className={cn(
          "min-w-0 flex-1 truncate",
          getUiTypographyClassName({ role: "caption", tone: "default", weight: "medium" }),
        )}>
          {title || toolUse.name}
        </span>
        {loading ? (
          <span
            aria-hidden="true"
            className="h-1.5 w-1.5 rounded-full bg-(--icon-muted) motion-safe:animate-pulse"
          />
        ) : visibleStatus === "error" ? (
          <span
            aria-hidden="true"
            className="h-1.5 w-1.5 rounded-full bg-(--destructive)"
          />
        ) : null}
      </header>
      {missingWidgetCode ? (
        <UiResourceState
          className="min-h-0 rounded-none border-x-0 px-4 py-5"
          impact={t("generative_ui.failure_impact")}
          nextStep={t("generative_ui.missing_next_step")}
          size="sm"
          state="error"
          title={t("generative_ui.missing_title")}
          urgency="polite"
          variant="card"
        />
      ) : renderState.status === "error" ? (
        <UiResourceState
          className="min-h-0 rounded-none border-x-0 px-4 py-5"
          impact={t("generative_ui.failure_impact")}
          primaryAction={{
            icon: <RotateCcw className="h-3.5 w-3.5" />,
            label: t("generative_ui.retry"),
            onClick: sendWidgetUpdate,
          }}
          size="sm"
          state="error"
          title={t("generative_ui.render_failed_title")}
          urgency="polite"
          variant="card"
        />
      ) : null}
      {widgetCode ? (
        <iframe
          className="block w-full border-0 bg-(--surface-panel-background)"
          loading="lazy"
          onLoad={sendWidgetUpdate}
          ref={frameRef}
          sandbox="allow-scripts"
          srcDoc={shellDocument}
          style={{ height }}
          title={title || toolUse.name}
        />
      ) : !complete ? (
        <UiSkeleton className="h-[180px] w-full surface-radius-sm" />
      ) : null}
    </section>
  );
}
