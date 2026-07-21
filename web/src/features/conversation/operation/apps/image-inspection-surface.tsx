"use client";

/**
 * INPUT: A non-workspace ViewImage source and the real tool result.
 * OUTPUT: A Preview-like image surface with the source image when embeddable and its analysis.
 * POS: ViewImage app UI; workspace files continue through the shared workspace preview router.
 */
import { Eye, EyeOff, FileImage, LoaderCircle, ScanText } from "lucide-react";
import { useState } from "react";

import { cn } from "@/shared/ui/class-name";

import type { OperationImageSourceKind } from "../operation-image-source";
import type { NexusOperationEvent } from "../operation-types";

const PHASE_LABEL = {
  queued: "等待分析",
  running: "正在分析",
  waiting: "等待确认",
  done: "分析完成",
  error: "分析失败",
  cancelled: "已中断",
} as const;

export function ImageInspectionSurface({
  event,
  preview,
  source,
  sourceKind,
}: {
  event: NexusOperationEvent;
  preview: unknown;
  source: string;
  sourceKind: OperationImageSourceKind;
}) {
  const [image_failed, set_image_failed] = useState(false);
  const [image_loaded, set_image_loaded] = useState(false);
  const image_src = sourceKind === "remote" || sourceKind === "inline" ? source : null;
  const question = typeof event.input_preview?.question === "string"
    ? event.input_preview.question.trim()
    : "";
  const analysis = extract_image_analysis_lines(
    preview ?? event.result_preview ?? event.summary,
  ).slice(0, 24);
  const analysis_lines = analysis.length > 0
    ? analysis
    : [fallback_analysis_text(event)];

  return (
    <div
      className="flex h-full min-h-[240px] min-w-0 flex-col overflow-hidden bg-[#edf1f4]"
      data-image-source-kind={sourceKind}
      data-stage-image-inspection
    >
      <header className="flex min-w-0 items-center gap-3 border-b border-black/8 bg-white/88 px-4 py-2.5">
        <span className="grid h-7 w-7 shrink-0 place-items-center rounded-[7px] border border-black/8 bg-white text-[#4c6070]">
          <FileImage className="h-4 w-4" />
        </span>
        <div className="min-w-0 flex-1">
          <p className="truncate text-[12px] font-semibold text-[#162230]">{source_title(source, sourceKind)}</p>
          {question ? <p className="truncate text-[10px] text-[#73808d]">{question}</p> : null}
        </div>
        <span className={cn(
          "flex shrink-0 items-center gap-1.5 text-[10px] font-semibold",
          event.phase === "error" ? "text-[#c75454]" : "text-[#71808d]",
        )}>
          {event.phase === "running" ? <LoaderCircle className="h-3 w-3 animate-spin" /> : <Eye className="h-3 w-3" />}
          {PHASE_LABEL[event.phase]}
        </span>
      </header>

      <div className="grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[minmax(0,1fr)_minmax(190px,32%)]">
        <div className="grid min-h-[180px] min-w-0 place-items-center overflow-auto border-b border-black/8 p-5 lg:border-b-0 lg:border-r">
          {image_src && !image_failed ? (
            <div className="relative grid max-h-full max-w-full place-items-center rounded-[8px] border border-black/10 bg-white p-3 shadow-[0_12px_28px_rgba(24,36,48,0.12)]">
              <img
                alt={source_title(source, sourceKind)}
                className={cn("max-h-[56vh] max-w-full object-contain", !image_loaded && "opacity-0")}
                src={image_src}
                onError={() => set_image_failed(true)}
                onLoad={() => set_image_loaded(true)}
              />
              {!image_loaded ? <LoaderCircle className="absolute h-5 w-5 animate-spin text-[#7b8995]" /> : null}
            </div>
          ) : (
            <div className="max-w-[280px] text-center">
              {image_failed ? <EyeOff className="mx-auto h-8 w-8 text-[#9a6670]" /> : <FileImage className="mx-auto h-8 w-8 text-[#82909c]" />}
              <p className="mt-3 text-[12px] font-semibold text-[#24313d]">
                {image_failed ? "图片无法载入" : source_placeholder_title(sourceKind)}
              </p>
              <p className="mt-1 text-[10px] leading-5 text-[#71808d]">
                {source_placeholder_detail(sourceKind)}
              </p>
            </div>
          )}
        </div>

        <aside className="min-h-0 overflow-auto bg-white/76 p-4">
          <div className="mb-3 flex items-center gap-2 text-[11px] font-semibold text-[#24313d]">
            <ScanText className="h-3.5 w-3.5 text-[#647684]" />
            视觉分析
          </div>
          <div className="space-y-2 text-[11px] leading-5 text-[#4e5e6b]" data-stage-image-analysis>
            {analysis_lines.map((line, index) => (
              <p className="whitespace-pre-wrap break-words" key={`${index}:${line}`}>{line}</p>
            ))}
          </div>
        </aside>
      </div>
    </div>
  );
}

function source_title(source: string, kind: OperationImageSourceKind): string {
  if (kind === "inline") return "内联图片";
  if (kind === "attachment") return "会话图片";
  if (kind === "unavailable") return "图像来源";
  try {
    const url = new URL(source);
    return decodeURIComponent(url.pathname).split("/").filter(Boolean).at(-1) || url.hostname;
  } catch {
    return source.split(/[\\/]/).filter(Boolean).at(-1) ?? "图片";
  }
}

function source_placeholder_title(kind: OperationImageSourceKind): string {
  if (kind === "attachment") return "会话图片分析";
  if (kind === "unavailable") return "无法直接预览此来源";
  return "图片预览";
}

function source_placeholder_detail(kind: OperationImageSourceKind): string {
  if (kind === "attachment") return "原始图片由当前运行会话托管，此处保留工具返回的视觉分析。";
  if (kind === "unavailable") return "该来源不能安全地嵌入舞台，此处展示工具的真实执行结果。";
  return "工具完成后将在这里显示图像。";
}

function fallback_analysis_text(event: NexusOperationEvent): string {
  if (event.phase === "running" || event.phase === "queued") return "视觉模型正在分析图像。";
  if (event.phase === "error") return event.summary || "图像分析失败。";
  return "工具没有返回可展示的视觉分析。";
}

function extract_image_analysis_lines(value: unknown, depth = 0): string[] {
  if (value == null || depth > 6) {
    return [];
  }
  if (typeof value === "string") {
    const parsed = parse_json_envelope(value);
    if (parsed !== null) {
      return extract_image_analysis_lines(parsed, depth + 1);
    }
    return value
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter(Boolean);
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return [String(value)];
  }
  if (Array.isArray(value)) {
    return value.flatMap((item) => extract_image_analysis_lines(item, depth + 1));
  }
  if (typeof value !== "object") {
    return [];
  }

  const record = value as Record<string, unknown>;
  const preferred_keys = [
    "content",
    "text",
    "analysis",
    "summary",
    "result",
    "output",
    "message",
  ] as const;
  for (const key of preferred_keys) {
    const lines = extract_image_analysis_lines(record[key], depth + 1);
    if (lines.length > 0) {
      return lines;
    }
  }
  return [];
}

function parse_json_envelope(value: string): unknown | null {
  const trimmed = value.trim();
  if (!trimmed || !["{", "[", '"'].includes(trimmed[0] ?? "")) {
    return null;
  }
  try {
    const parsed = JSON.parse(trimmed) as unknown;
    return typeof parsed === "string" && parsed === value ? null : parsed;
  } catch {
    return null;
  }
}
