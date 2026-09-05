// INPUT: 已选定的二维码 payload、可选失败内容与载荷可见性。
// OUTPUT: 使用共享表面、形状与排版的二维码加载、成功或失败投影。
// POS: shared/ui 二维码原语；不解释登录、授权协议或 payload 业务含义。
"use client";

import { useEffect, type ReactNode } from "react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { cn } from "@/shared/ui/class-name";
import { UiPanel } from "@/shared/ui/panel";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export function UiQRCode({
  alt,
  failureFallback,
  loadingLabel = "正在生成二维码…",
  payload,
  showPayload = true,
}: {
  alt: string;
  failureFallback?: ReactNode;
  loadingLabel?: string;
  payload: string;
  showPayload?: boolean;
}) {
  const value = payload.trim();
  const embeddedImage = value.startsWith("data:image/");
  const [generation, setGeneration] = useResettableState<{
    imageUrl: string;
    status: "failed" | "idle" | "loading" | "ready";
  }>({
    imageUrl: "",
    status: value && !embeddedImage ? "loading" : "idle",
  }, value);
  const imageUrl = embeddedImage
    ? value
    : generation.status === "ready"
      ? generation.imageUrl
      : "";

  useEffect(() => {
    if (!value || embeddedImage) {
      return;
    }
    let cancelled = false;
    void import("qrcode")
      .then((module) => module.toDataURL(value, {
        errorCorrectionLevel: "M",
        margin: 1,
        scale: 7,
        width: 220,
      }))
      .then((url) => {
        if (!cancelled) {
          setGeneration(url
            ? { imageUrl: url, status: "ready" }
            : { imageUrl: "", status: "failed" });
        }
      })
      .catch(() => {
        if (!cancelled) {
          setGeneration({ imageUrl: "", status: "failed" });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [embeddedImage, setGeneration, value]);

  if (!value) {
    return null;
  }

  return (
    <UiPanel className="flex flex-col items-center gap-2" padding="md" radius="md">
      {imageUrl ? (
        <img
          alt={alt}
          className="surface-radius-sm h-[220px] w-[220px] bg-(--surface-paper-background) p-2"
          src={imageUrl}
        />
      ) : generation.status === "loading" ? (
        <div
          aria-live="polite"
          className={cn(
            "surface-radius-sm flex h-[220px] w-[220px] items-center justify-center bg-(--surface-paper-background) p-4 text-center text-(--surface-paper-muted)",
            getUiTypographyClassName({ role: "metadata" }),
          )}
          role="status"
        >
          {loadingLabel}
        </div>
      ) : (
        <div className={cn(
          "surface-radius-sm flex min-h-[220px] w-[220px] items-center justify-center bg-(--surface-paper-background) p-4 text-center text-(--surface-paper-muted)",
          getUiTypographyClassName({ role: "metadata" }),
        )}>
          {failureFallback
            ?? (showPayload
              ? "二维码生成失败，请使用下方链接"
              : "二维码生成失败，请重新发起授权")}
        </div>
      )}
      {showPayload ? (
        <code className={cn(
          "surface-radius-sm max-w-full truncate border border-(--divider-subtle-color) px-2 py-1",
          getUiTypographyClassName({ role: "code", tone: "muted" }),
        )}>
          {payload}
        </code>
      ) : null}
    </UiPanel>
  );
}
