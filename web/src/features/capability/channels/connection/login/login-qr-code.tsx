"use client";

import { useEffect } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";

export function LoginQRCode({ payload }: { payload: string }) {
  const value = payload.trim();
  const [generatedImageUrl, setGeneratedImageUrl] = useResettableState("", value);
  const imageUrl = value.startsWith("data:image/") ? value : generatedImageUrl;

  useEffect(() => {
    if (!value || value.startsWith("data:image/")) {
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
          setGeneratedImageUrl(url);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setGeneratedImageUrl("");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [setGeneratedImageUrl, value]);

  if (!value) {
    return null;
  }

  return (
    <div className="flex flex-col items-center gap-2 rounded-[12px] border border-(--divider-subtle-color) px-4 py-4">
      {imageUrl ? (
        <img
          alt="微信扫码登录二维码"
          className="h-[220px] w-[220px] rounded-[8px] bg-(--surface-paper-background) p-2"
          src={imageUrl}
        />
      ) : (
        <div className="flex h-[220px] w-[220px] items-center justify-center rounded-[8px] bg-(--surface-paper-background) p-4 text-center text-[12px] leading-5 text-(--surface-paper-muted)">
          二维码生成失败，请使用下方链接
        </div>
      )}
      <code className="max-w-full truncate rounded-[8px] border border-(--divider-subtle-color) px-2 py-1 text-[11px] text-(--text-muted)">
        {payload}
      </code>
    </div>
  );
}
