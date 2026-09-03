// INPUT: Agent 配置自动保存的阶段与用户可见状态文案。
// OUTPUT: 复用共享图标动作、Spinner、Typography 与浮层 token 的保存状态。
// POS: Contacts Agent 详情头部状态组件；不执行保存或解释失败原因。

"use client";

import { useEffect, useState } from "react";

import { Check, CircleAlert, LoaderCircle } from "lucide-react";

import type { AgentOptionsPersistenceState } from "@/features/agents/options/agent-options-editor-model";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { getUiOverlayLayerClassName } from "@/shared/ui/overlay/layer-styles";
import { OVERLAY_SURFACE_CLASS_NAME } from "@/shared/ui/overlay/overlay-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export function AgentOptionsPersistenceStatus({
  state,
}: {
  state: AgentOptionsPersistenceState;
}) {
  const [mobileErrorOpen, setMobileErrorOpen] = useState(false);
  useEffect(() => {
    setMobileErrorOpen(false);
  }, [state.message, state.phase]);

  const StatusIcon = state.phase === "saving"
    ? LoaderCircle
    : state.phase === "success"
      ? Check
      : state.phase === "error"
        ? CircleAlert
        : null;

  return (
    <span
      aria-live="polite"
      className={cn(
        "relative mr-1 inline-flex h-8 shrink-0 items-center gap-1",
        getUiTypographyClassName({ role: "caption", tone: "soft" }),
        state.phase === "success" && "text-(--success)",
        state.phase === "error" && "text-(--destructive)",
      )}
      title={state.message}
    >
      {StatusIcon && state.phase === "error" ? (
        <>
          <UiIconButton
            aria-expanded={mobileErrorOpen}
            aria-label={state.message}
            className="sm:hidden"
            data-agent-save-error-details
            onClick={() => setMobileErrorOpen((current) => !current)}
            size="md"
            tone="danger"
            variant="ghost"
          >
            <StatusIcon aria-hidden="true" className="h-3.5 w-3.5" />
          </UiIconButton>
          <StatusIcon aria-hidden="true" className="h-3.5 w-3.5 max-sm:hidden" />
        </>
      ) : StatusIcon ? (
        <StatusIcon
          aria-hidden="true"
          className={state.phase === "saving"
            ? getUiSpinnerClassName({ size: "sm" })
            : "h-3.5 w-3.5"}
        />
      ) : null}
      <span
        aria-hidden={state.phase === "error" ? "true" : undefined}
        className="sr-only sm:not-sr-only"
      >
        {state.message}
      </span>
      {state.phase === "error" && mobileErrorOpen ? (
        <span
          aria-hidden="true"
          className={cn(
            "absolute right-0 top-[calc(100%+0.375rem)] w-[min(19rem,calc(100vw-1.5rem))] border border-[color:color-mix(in_srgb,var(--destructive)_20%,transparent)] px-3 py-2.5 text-left sm:hidden",
            getUiOverlayLayerClassName("popover"),
            OVERLAY_SURFACE_CLASS_NAME,
            getUiTypographyClassName({ role: "caption", tone: "default" }),
          )}
          data-agent-save-error-popover
        >
          {state.message}
        </span>
      ) : null}
    </span>
  );
}
