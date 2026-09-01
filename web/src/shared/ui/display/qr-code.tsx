"use client";

import { useEffect, type ReactNode } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";

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
    <div className="flex flex-col items-center gap-2 rounded-[12px] border border-(--divider-subtle-color) px-4 py-4">
      {imageUrl ? (
        <img
          alt={alt}
          className="h-[220px] w-[220px] rounded-[8px] bg-(--surface-paper-background) p-2"
          src={imageUrl}
        />
      ) : generation.status === "loading" ? (
        <div
          aria-live="polite"
          className="flex h-[220px] w-[220px] items-center justify-center rounded-[8px] bg-(--surface-paper-background) p-4 text-center text-compact leading-5 text-(--surface-paper-muted)"
          role="status"
        >
          {loadingLabel}
        </div>
      ) : (
        <div className="flex min-h-[220px] w-[220px] items-center justify-center rounded-[8px] bg-(--surface-paper-background) p-4 text-center text-compact leading-5 text-(--surface-paper-muted)">
          {failureFallback
            ?? (showPayload
              ? "二维码生成失败，请使用下方链接"
              : "二维码生成失败，请重新发起授权")}
        </div>
      )}
      {showPayload ? (
        <code className="max-w-full truncate rounded-[8px] border border-(--divider-subtle-color) px-2 py-1 text-xs text-(--text-muted)">
          {payload}
        </code>
      ) : null}
    </div>
  );
}
