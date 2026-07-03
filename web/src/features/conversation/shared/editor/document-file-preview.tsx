"use client";

import { type CSSProperties, useCallback, useEffect, useRef, useState } from "react";
import { Eye, FileText, FileWarning, LoaderCircle } from "lucide-react";
import type { Options as DocxPreviewOptions } from "docx-preview";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { getWorkspaceFilePreviewUrl } from "@/lib/api/agent-manage-api";
import { cn } from "@/lib/utils";
import { ConversationResizeHandle } from "./conversation-resize-handle";
import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
} from "./workspace-file-preview-chrome";

const MAX_DOCX_PREVIEW_BYTES = 15 * 1024 * 1024;

type DocumentPreviewStatus =
  | { state: "loading"; message: string }
  | { state: "loaded" }
  | { state: "error"; message: string };

interface DocumentFilePreviewProps {
  agentId: string;
  embedded?: boolean;
  fileName: string;
  isPreviewFocused?: boolean;
  onResizeStart: () => void;
  onTogglePreviewFocus?: () => void;
  path: string;
}

const DOCX_RENDER_OPTIONS: Partial<DocxPreviewOptions> = {
  breakPages: true,
  className: "nexus-docx-preview",
  debug: false,
  experimental: false,
  ignoreFonts: false,
  ignoreHeight: false,
  ignoreLastRenderedPageBreak: false,
  ignoreWidth: false,
  inWrapper: true,
  renderAltChunks: false,
  renderChanges: false,
  renderComments: false,
  renderEndnotes: true,
  renderFooters: true,
  renderFootnotes: true,
  renderHeaders: true,
  trimXmlDeclaration: true,
  useBase64URL: true,
};

export function DocumentFilePreview({
  agentId: agentId,
  embedded,
  fileName: fileName,
  isPreviewFocused: isPreviewFocused,
  onResizeStart: onResizeStart,
  onTogglePreviewFocus: onTogglePreviewFocus,
  path,
}: DocumentFilePreviewProps) {
  const viewportRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const styleContainerRef = useRef<HTMLDivElement>(null);
  const previewKey = `${agentId}\x1f${path}`;
  const [previewScale, setPreviewScale] = useResettableState(1, previewKey);
  const [status, setStatus] = useResettableState<DocumentPreviewStatus>({
    state: "loading",
    message: "加载文档预览中",
  }, previewKey);

  const updatePreviewScale = useCallback(() => {
    const viewport = viewportRef.current;
    const container = containerRef.current;
    if (!viewport || !container) {
      return;
    }

    const pageWidth = getDocxPageWidth(container);
    if (pageWidth <= 0) {
      setPreviewScale(1);
      return;
    }

    const availableWidth = Math.max(viewport.clientWidth - 40, 1);
    const nextScale = Math.max(Math.min(availableWidth / pageWidth, 1), 0.1);
    const roundedScale = Math.round(nextScale * 1000) / 1000;
    setPreviewScale((current) => (
      Math.abs(current - roundedScale) > 0.005 ? roundedScale : current
    ));
  }, []);

  useEffect(() => {
    const container = containerRef.current;
    const styleContainer = styleContainerRef.current;
    const abortController = new AbortController();
    let cancelled = false;

    if (container) {
      container.innerHTML = "";
    }
    if (styleContainer) {
      styleContainer.innerHTML = "";
    }

    async function loadPreview() {
      if (!container || !styleContainer) {
        return;
      }

      try {
        const previewUrl = getWorkspaceFilePreviewUrl(agentId, path);
        const response = await fetch(previewUrl, {
          credentials: "include",
          signal: abortController.signal,
        });

        if (!response.ok) {
          throw new Error(`读取失败: ${response.status}`);
        }

        const contentLength = response.headers.get("content-length");
        if (contentLength && Number(contentLength) > MAX_DOCX_PREVIEW_BYTES) {
          throw new Error("docx 文件超过 15MB，当前无法内置预览，请使用上方按钮处理");
        }

        const buffer = await response.arrayBuffer();
        if (cancelled) {
          return;
        }
        if (buffer.byteLength > MAX_DOCX_PREVIEW_BYTES) {
          throw new Error("docx 文件超过 15MB，当前无法内置预览，请使用上方按钮处理");
        }

        setStatus({ state: "loading", message: "解析 docx 文件中" });
        const { renderAsync } = await import("docx-preview");
        if (cancelled) {
          return;
        }

        await renderAsync(buffer, container, styleContainer, DOCX_RENDER_OPTIONS);
        if (cancelled) {
          return;
        }

        normalizeDocxMedia(container);
        updatePreviewScale();
        requestAnimationFrame(() => {
          normalizeDocxMedia(container);
          updatePreviewScale();
        });
        setStatus({ state: "loaded" });
      } catch (previewError) {
        if (cancelled || abortController.signal.aborted) {
          return;
        }
        const message = previewError instanceof Error ? previewError.message : "docx 预览失败";
        if (container) {
          container.innerHTML = "";
        }
        if (styleContainer) {
          styleContainer.innerHTML = "";
        }
        setPreviewScale(1);
        setStatus({ state: "error", message });
      }
    }

    void loadPreview();

    return () => {
      cancelled = true;
      abortController.abort();
      if (container) {
        container.innerHTML = "";
      }
      if (styleContainer) {
        styleContainer.innerHTML = "";
      }
      setPreviewScale(1);
    };
  }, [agentId, path, updatePreviewScale]);

  useEffect(() => {
    if (status.state !== "loaded") {
      return;
    }

    const viewport = viewportRef.current;
    const container = containerRef.current;
    if (!viewport || !container) {
      return;
    }

    const observer = new ResizeObserver(updatePreviewScale);
    observer.observe(viewport);
    observer.observe(container);
    window.addEventListener("resize", updatePreviewScale);
    updatePreviewScale();

    return () => {
      observer.disconnect();
      window.removeEventListener("resize", updatePreviewScale);
    };
  }, [status.state, updatePreviewScale]);

  const isLoading = status.state === "loading";
  const isLoaded = status.state === "loaded";
  const hasError = status.state === "error";
  const hostStyle = {
    "--docx-preview-scale": String(previewScale),
  } as CSSProperties;

  return (
    <>
      {!embedded ? (
        <ConversationResizeHandle
          ariaLabel="调整编辑器宽度"
          className="flex"
          onMouseDown={onResizeStart}
        />
      ) : null}

      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton agentId={agentId} fileName={fileName} path={path} />
            <WorkspaceFilePreviewFocusButton
              isPreviewFocused={isPreviewFocused}
              onTogglePreviewFocus={onTogglePreviewFocus}
            />
          </>
        )}
        embedded={embedded}
        meta={(
          <>
            <span className="flex items-center gap-1">
              <FileText className="h-3 w-3" />
              docx 预览
            </span>
            {hasError ? (
              <span className="flex items-center gap-1 text-destructive">
                <FileWarning className="h-3 w-3" />
                加载失败
              </span>
            ) : isLoaded ? (
              <span className="flex items-center gap-1 text-(--success)">
                <Eye className="h-3 w-3" />
                已加载
              </span>
            ) : (
              <span className="flex items-center gap-1">
                <LoaderCircle className="h-3 w-3 animate-spin" />
                {isLoading ? status.message : "加载中"}
              </span>
            )}
          </>
        )}
        title={fileName}
      />

      <div
        ref={viewportRef}
        className="soft-scrollbar relative min-h-0 flex-1 overflow-auto bg-[var(--surface-panel-subtle-background)] p-5"
      >
        <style>
          {`
            .nexus-docx-preview-host .nexus-docx-preview-wrapper {
              align-items: center;
              background: transparent !important;
              box-sizing: border-box;
              display: flex;
              flex-direction: column;
              gap: 18px;
              max-width: none;
              min-width: 0;
              padding: 0 !important;
              zoom: var(--docx-preview-scale, 1);
            }

            .nexus-docx-preview-host section.nexus-docx-preview {
              background: #ffffff;
              box-shadow: 0 18px 36px rgba(15, 23, 42, 0.14);
              box-sizing: border-box;
              color: #111827;
              overflow: hidden;
            }

            .nexus-docx-preview-host section.nexus-docx-preview table {
              border-collapse: collapse;
            }

            .nexus-docx-preview-host section.nexus-docx-preview img,
            .nexus-docx-preview-host section.nexus-docx-preview svg {
              height: auto !important;
              max-width: 100% !important;
              object-fit: contain;
            }
          `}
        </style>
        <div ref={styleContainerRef} aria-hidden="true" className="contents" />
        {hasError ? (
          <div className="flex h-full min-h-[240px] items-center justify-center text-center">
            <div className="max-w-sm">
              <FileWarning className="mx-auto h-12 w-12 text-(--icon-muted)" />
              <p className="mt-4 text-sm font-medium text-(--text-strong)">docx 预览失败</p>
              <p className="mt-2 text-xs leading-5 text-(--text-soft)">{status.message}</p>
            </div>
          </div>
        ) : (
          <div
            ref={containerRef}
            className={cn(
              "nexus-docx-preview-host mx-auto flex min-h-full w-full min-w-0 justify-center",
              isLoaded ? "opacity-100" : "opacity-0",
            )}
            style={hostStyle}
          />
        )}
        {isLoading ? (
          <div className="absolute inset-x-0 top-24 flex justify-center pointer-events-none">
            <div className="inline-flex items-center gap-2 rounded-full border border-(--divider-subtle-color) bg-(--surface-panel-background) px-3 py-1.5 text-xs text-(--text-muted) shadow-sm">
              <LoaderCircle className="h-3.5 w-3.5 animate-spin" />
              <span>{status.message}</span>
            </div>
          </div>
        ) : null}
      </div>
    </>
  );
}

function getDocxPageWidth(container: HTMLElement): number {
  const pages = Array.from(container.querySelectorAll<HTMLElement>("section.nexus-docx-preview"));
  return pages.reduce((maxWidth, page) => {
    const cssWidth = parseCssLengthToPx(page.style.width);
    const layoutWidth = page.offsetWidth || page.clientWidth || page.getBoundingClientRect().width;
    return Math.max(maxWidth, cssWidth, layoutWidth, page.scrollWidth);
  }, 0);
}

function normalizeDocxMedia(container: HTMLElement) {
  const mediaElements = Array.from(container.querySelectorAll<HTMLElement>("section.nexus-docx-preview img, section.nexus-docx-preview svg"));
  mediaElements.forEach((media) => {
    const page = media.closest<HTMLElement>("section.nexus-docx-preview");
    if (!page) {
      return;
    }

    const mediaWidth = getUnscaledElementWidth(media);
    const widthLimit = getDocxMediaWidthLimit(media, page, mediaWidth);
    if (mediaWidth <= 0 || widthLimit <= 0 || mediaWidth <= widthLimit + 1) {
      return;
    }

    const nextWidth = `${Math.floor(widthLimit)}px`;
    media.style.width = nextWidth;
    media.style.maxWidth = "100%";
    media.style.height = "auto";
    media.style.objectFit = "contain";

    const parent = media.parentElement;
    if (parent && parent !== page) {
      parent.style.width = nextWidth;
      parent.style.maxWidth = "100%";
      parent.style.height = "auto";
      parent.style.overflow = "hidden";
    }
  });
}

function getDocxMediaWidthLimit(media: HTMLElement, page: HTMLElement, mediaWidth: number): number {
  const pageStyle = window.getComputedStyle(page);
  const pageContentWidth = page.clientWidth
    - parseCssLengthToPx(pageStyle.paddingLeft)
    - parseCssLengthToPx(pageStyle.paddingRight);
  const candidates = [pageContentWidth].filter((width) => width > 0);
  let current = media.parentElement;

  while (current && current !== page) {
    const style = window.getComputedStyle(current);
    if (style.display !== "inline" && current.clientWidth > 0 && current.clientWidth < mediaWidth) {
      candidates.push(current.clientWidth);
    }
    current = current.parentElement;
  }

  return Math.max(Math.min(...candidates), 120);
}

function getUnscaledElementWidth(element: HTMLElement): number {
  return parseCssLengthToPx(element.style.width)
    || Number(element.getAttribute("width") || 0)
    || element.scrollWidth
    || element.offsetWidth
    || element.getBoundingClientRect().width;
}

function parseCssLengthToPx(value: string): number {
  const match = value.trim().match(/^(-?\d+(?:\.\d+)?)(px|pt|in|cm|mm)?$/i);
  if (!match) {
    return 0;
  }

  const amount = Number(match[1]);
  const unit = match[2]?.toLowerCase() || "px";
  switch (unit) {
    case "cm":
      return (amount * 96) / 2.54;
    case "in":
      return amount * 96;
    case "mm":
      return (amount * 96) / 25.4;
    case "pt":
      return (amount * 96) / 72;
    default:
      return amount;
  }
}
