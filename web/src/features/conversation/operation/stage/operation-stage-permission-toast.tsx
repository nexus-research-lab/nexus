import { useEffect, useMemo, useRef, useState } from "react";
import { ChevronDown, LockKeyhole, ShieldCheck, ShieldX, X } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import type { PermissionDecisionPayload } from "@/types/conversation/interaction/permission";

import type { NexusOperationEvent } from "../operation-types";
import {
  findWaitingPermissionEvent,
  readPermissionCommand,
  readPermissionSummary,
} from "./operation-stage-permission-model";

export function OperationStagePermissionToast({
  event,
  events,
  onPermissionResponse,
}: {
  event: NexusOperationEvent;
  events: NexusOperationEvent[];
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
}) {
  const permissionEvent = useMemo(
    () => findWaitingPermissionEvent(event, events),
    [event, events],
  );
  const [menuOpen, setMenuOpen] = useState(false);
  const [dismissedRequestId, setDismissedRequestId] = useState<string | null>(null);
  const [submittedRequestId, setSubmittedRequestId] = useState<string | null>(null);
  const [responseError, setResponseError] = useState<string | null>(null);
  const submittingRequestIdRef = useRef<string | null>(null);
  const requestId = permissionEvent?.permission_request_id ?? null;

  useEffect(() => {
    setMenuOpen(false);
    setDismissedRequestId(null);
    setSubmittedRequestId(null);
    setResponseError(null);
    submittingRequestIdRef.current = null;
  }, [requestId]);

  if (
    !permissionEvent
    || !requestId
    || dismissedRequestId === requestId
    || submittedRequestId === requestId
  ) {
    return null;
  }

  const isQuestion = permissionEvent.permission_interaction_mode === "question";
  const canDecide = Boolean(onPermissionResponse && !isQuestion);
  const command = readPermissionCommand(permissionEvent);
  const summary = readPermissionSummary(permissionEvent, command);

  const dismiss = () => {
    setDismissedRequestId(requestId);
    setMenuOpen(false);
  };

  const submitDecision = (decision: "allow" | "deny") => {
    if (
      !onPermissionResponse
      || isQuestion
      || submittingRequestIdRef.current === requestId
    ) {
      return;
    }
    submittingRequestIdRef.current = requestId;
    setResponseError(null);
    const accepted = onPermissionResponse({ request_id: requestId, decision });
    if (!accepted) {
      submittingRequestIdRef.current = null;
      setResponseError("未能提交，请检查连接后重试");
      return;
    }
    setSubmittedRequestId(requestId);
    setMenuOpen(false);
  };

  return (
    <div className="pointer-events-none absolute right-3 top-12 z-[190] w-[min(380px,calc(100%-24px))] md:right-5 md:top-14 md:w-[min(380px,calc(100%-40px))]">
      <div className="pointer-events-auto overflow-visible rounded-[22px] border border-white/70 bg-[rgba(244,248,251,0.78)] p-3 text-(--text-strong) shadow-[0_22px_62px_rgba(18,28,42,0.18),inset_0_1px_0_rgba(255,255,255,0.78)] backdrop-blur-2xl">
        <div className="flex items-start gap-2.5">
          <span className="grid h-10 w-10 shrink-0 place-items-center rounded-[13px] border border-white/70 bg-[linear-gradient(135deg,#f8fafc,#dbeafe_54%,#5b72ff)] text-white shadow-[0_9px_20px_rgba(91,114,255,0.20),inset_0_1px_0_rgba(255,255,255,0.74)]">
            <LockKeyhole className="h-5 w-5 drop-shadow" />
          </span>

          <div className="min-w-0 flex-1">
            <div className="flex items-start justify-between gap-2">
              <p className="min-w-0 text-[13px] font-black leading-5 tracking-normal">
                {summary}
              </p>
              <button
                aria-label="关闭权限通知"
                className="grid h-6 w-6 shrink-0 place-items-center rounded-full text-(--text-muted) transition hover:bg-white/58 hover:text-(--text-strong) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.34)]"
                onClick={dismiss}
                type="button"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>

            {command ? (
              <p className="mt-2 max-h-20 overflow-auto whitespace-pre-wrap break-words rounded-[9px] bg-[rgba(15,23,42,0.08)] px-2.5 py-1.5 font-mono text-[10px] font-semibold leading-4 text-(--text-muted) [overflow-wrap:anywhere]">
                {command}
              </p>
            ) : null}

            <div className="mt-2.5 flex items-center justify-between gap-2">
              <span className={cn(
                "rounded-full bg-white/58 px-2.5 py-1 text-[9px] font-black",
                responseError ? "text-[#b64e52]" : "text-(--text-soft)",
              )}>
                {responseError ?? (isQuestion ? "等待输入" : "等待确认")}
              </span>

              {isQuestion ? null : (
                <div className="relative flex items-center overflow-visible rounded-full bg-white/64 shadow-[inset_0_0_0_1px_rgba(255,255,255,0.65)]">
                  <button
                    aria-label="允许此请求"
                    className="inline-flex h-8 items-center gap-1.5 rounded-l-full px-3 text-[12px] font-black text-[color:var(--primary)] transition enabled:hover:bg-[rgba(91,114,255,0.10)] disabled:cursor-not-allowed disabled:text-(--text-muted) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.34)]"
                    disabled={!canDecide}
                    onClick={() => submitDecision("allow")}
                    type="button"
                  >
                    <ShieldCheck className="h-4 w-4" />
                    允许
                  </button>
                  <span className="h-4 w-px bg-[rgba(100,116,139,0.16)]" />
                  <button
                    aria-expanded={menuOpen}
                    aria-label="更多权限选项"
                    className={cn(
                      "grid h-8 w-8 place-items-center rounded-r-full text-(--text-muted) transition hover:bg-white/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.34)]",
                      menuOpen && "bg-[rgba(91,114,255,0.12)] text-[color:var(--primary)]",
                    )}
                    onClick={() => setMenuOpen((open) => !open)}
                    title="更多权限选项"
                    type="button"
                  >
                    <ChevronDown className={cn("h-4 w-4 transition-transform", menuOpen && "rotate-180")} />
                  </button>

                  {menuOpen ? (
                    <div className="absolute right-0 top-[calc(100%+8px)] w-[156px] overflow-hidden rounded-[14px] border border-white/72 bg-[rgba(248,250,252,0.94)] p-1.5 text-[12px] font-bold shadow-[0_20px_48px_rgba(18,28,42,0.18),inset_0_1px_0_rgba(255,255,255,0.82)] backdrop-blur-2xl">
                      <button
                        aria-label="拒绝此请求"
                        className="flex h-9 w-full items-center gap-2 rounded-[10px] px-2.5 text-left text-[#b64e52] transition enabled:hover:bg-[rgba(239,68,68,0.10)] disabled:cursor-not-allowed disabled:text-(--text-muted)"
                        disabled={!canDecide}
                        onClick={() => submitDecision("deny")}
                        type="button"
                      >
                        <ShieldX className="h-4 w-4" />
                        拒绝
                      </button>
                    </div>
                  ) : null}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
