"use client";

import { type CSSProperties, useCallback, useEffect, useRef, useState } from "react";
import { Eye, FileText, FileWarning, LoaderCircle } from "lucide-react";
import type { Options as DocxPreviewOptions } from "docx-preview";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { get_workspace_file_preview_url } from "@/lib/api/agent-manage-api";
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
  agent_id: string;
  embedded?: boolean;
  file_name: string;
  is_preview_focused?: boolean;
  on_resize_start: () => void;
  on_toggle_preview_focus?: () => void;
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
  agent_id,
  embedded,
  file_name,
  is_preview_focused,
  on_resize_start,
  on_toggle_preview_focus,
  path,
}: DocumentFilePreviewProps) {
  const viewport_ref = useRef<HTMLDivElement>(null);
  const container_ref = useRef<HTMLDivElement>(null);
  const style_container_ref = useRef<HTMLDivElement>(null);
  const preview_key = `${agent_id}\x1f${path}`;
  const [preview_scale, set_preview_scale] = useResettableState(1, preview_key);
  const [status, set_status] = useResettableState<DocumentPreviewStatus>({
    state: "loading",
    message: "加载文档预览中",
  }, preview_key);

  const update_preview_scale = useCallback(() => {
    const viewport = viewport_ref.current;
    const container = container_ref.current;
    if (!viewport || !container) {
      return;
    }

    const page_width = get_docx_page_width(container);
    if (page_width <= 0) {
      set_preview_scale(1);
      return;
    }

    const available_width = Math.max(viewport.clientWidth - 40, 1);
    const next_scale = Math.max(Math.min(available_width / page_width, 1), 0.1);
    const rounded_scale = Math.round(next_scale * 1000) / 1000;
    set_preview_scale((current) => (
      Math.abs(current - rounded_scale) > 0.005 ? rounded_scale : current
    ));
  }, []);

  useEffect(() => {
    const container = container_ref.current;
    const style_container = style_container_ref.current;
    const abort_controller = new AbortController();
    let cancelled = false;

    if (container) {
      container.innerHTML = "";
    }
    if (style_container) {
      style_container.innerHTML = "";
    }

    async function load_preview() {
      if (!container || !style_container) {
        return;
      }

      try {
        const preview_url = get_workspace_file_preview_url(agent_id, path);
        const response = await fetch(preview_url, {
          credentials: "include",
          signal: abort_controller.signal,
        });

        if (!response.ok) {
          throw new Error(`读取失败: ${response.status}`);
        }

        const content_length = response.headers.get("content-length");
        if (content_length && Number(content_length) > MAX_DOCX_PREVIEW_BYTES) {
          throw new Error("docx 文件超过 15MB，当前无法内置预览，请使用上方按钮处理");
        }

        const buffer = await response.arrayBuffer();
        if (cancelled) {
          return;
        }
        if (buffer.byteLength > MAX_DOCX_PREVIEW_BYTES) {
          throw new Error("docx 文件超过 15MB，当前无法内置预览，请使用上方按钮处理");
        }

        set_status({ state: "loading", message: "解析 docx 文件中" });
        const { renderAsync } = await import("docx-preview");
        if (cancelled) {
          return;
        }

        await renderAsync(buffer, container, style_container, DOCX_RENDER_OPTIONS);
        if (cancelled) {
          return;
        }

        normalize_docx_media(container);
        update_preview_scale();
        requestAnimationFrame(() => {
          normalize_docx_media(container);
          update_preview_scale();
        });
        set_status({ state: "loaded" });
      } catch (preview_error) {
        if (cancelled || abort_controller.signal.aborted) {
          return;
        }
        const message = preview_error instanceof Error ? preview_error.message : "docx 预览失败";
        if (container) {
          container.innerHTML = "";
        }
        if (style_container) {
          style_container.innerHTML = "";
        }
        set_preview_scale(1);
        set_status({ state: "error", message });
      }
    }

    void load_preview();

    return () => {
      cancelled = true;
      abort_controller.abort();
      if (container) {
        container.innerHTML = "";
      }
      if (style_container) {
        style_container.innerHTML = "";
      }
      set_preview_scale(1);
    };
  }, [agent_id, path, update_preview_scale]);

  useEffect(() => {
    if (status.state !== "loaded") {
      return;
    }

    const viewport = viewport_ref.current;
    const container = container_ref.current;
    if (!viewport || !container) {
      return;
    }

    const observer = new ResizeObserver(update_preview_scale);
    observer.observe(viewport);
    observer.observe(container);
    window.addEventListener("resize", update_preview_scale);
    update_preview_scale();

    return () => {
      observer.disconnect();
      window.removeEventListener("resize", update_preview_scale);
    };
  }, [status.state, update_preview_scale]);

  const is_loading = status.state === "loading";
  const is_loaded = status.state === "loaded";
  const has_error = status.state === "error";
  const host_style = {
    "--docx-preview-scale": String(preview_scale),
  } as CSSProperties;

  return (
    <>
      {!embedded ? (
        <ConversationResizeHandle
          aria_label="调整编辑器宽度"
          class_name="flex"
          on_mouse_down={on_resize_start}
        />
      ) : null}

      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton agent_id={agent_id} file_name={file_name} path={path} />
            <WorkspaceFilePreviewFocusButton
              is_preview_focused={is_preview_focused}
              on_toggle_preview_focus={on_toggle_preview_focus}
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
            {has_error ? (
              <span className="flex items-center gap-1 text-destructive">
                <FileWarning className="h-3 w-3" />
                加载失败
              </span>
            ) : is_loaded ? (
              <span className="flex items-center gap-1 text-(--success)">
                <Eye className="h-3 w-3" />
                已加载
              </span>
            ) : (
              <span className="flex items-center gap-1">
                <LoaderCircle className="h-3 w-3 animate-spin" />
                {is_loading ? status.message : "加载中"}
              </span>
            )}
          </>
        )}
        title={file_name}
      />

      <div
        ref={viewport_ref}
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
        <div ref={style_container_ref} aria-hidden="true" className="contents" />
        {has_error ? (
          <div className="flex h-full min-h-[240px] items-center justify-center text-center">
            <div className="max-w-sm">
              <FileWarning className="mx-auto h-12 w-12 text-(--icon-muted)" />
              <p className="mt-4 text-sm font-medium text-(--text-strong)">docx 预览失败</p>
              <p className="mt-2 text-xs leading-5 text-(--text-soft)">{status.message}</p>
            </div>
          </div>
        ) : (
          <div
            ref={container_ref}
            className={cn(
              "nexus-docx-preview-host mx-auto flex min-h-full w-full min-w-0 justify-center",
              is_loaded ? "opacity-100" : "opacity-0",
            )}
            style={host_style}
          />
        )}
        {is_loading ? (
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

function get_docx_page_width(container: HTMLElement): number {
  const pages = Array.from(container.querySelectorAll<HTMLElement>("section.nexus-docx-preview"));
  return pages.reduce((max_width, page) => {
    const css_width = parse_css_length_to_px(page.style.width);
    const layout_width = page.offsetWidth || page.clientWidth || page.getBoundingClientRect().width;
    return Math.max(max_width, css_width, layout_width, page.scrollWidth);
  }, 0);
}

function normalize_docx_media(container: HTMLElement) {
  const media_elements = Array.from(container.querySelectorAll<HTMLElement>("section.nexus-docx-preview img, section.nexus-docx-preview svg"));
  media_elements.forEach((media) => {
    const page = media.closest<HTMLElement>("section.nexus-docx-preview");
    if (!page) {
      return;
    }

    const media_width = get_unscaled_element_width(media);
    const width_limit = get_docx_media_width_limit(media, page, media_width);
    if (media_width <= 0 || width_limit <= 0 || media_width <= width_limit + 1) {
      return;
    }

    const next_width = `${Math.floor(width_limit)}px`;
    media.style.width = next_width;
    media.style.maxWidth = "100%";
    media.style.height = "auto";
    media.style.objectFit = "contain";

    const parent = media.parentElement;
    if (parent && parent !== page) {
      parent.style.width = next_width;
      parent.style.maxWidth = "100%";
      parent.style.height = "auto";
      parent.style.overflow = "hidden";
    }
  });
}

function get_docx_media_width_limit(media: HTMLElement, page: HTMLElement, media_width: number): number {
  const page_style = window.getComputedStyle(page);
  const page_content_width = page.clientWidth
    - parse_css_length_to_px(page_style.paddingLeft)
    - parse_css_length_to_px(page_style.paddingRight);
  const candidates = [page_content_width].filter((width) => width > 0);
  let current = media.parentElement;

  while (current && current !== page) {
    const style = window.getComputedStyle(current);
    if (style.display !== "inline" && current.clientWidth > 0 && current.clientWidth < media_width) {
      candidates.push(current.clientWidth);
    }
    current = current.parentElement;
  }

  return Math.max(Math.min(...candidates), 120);
}

function get_unscaled_element_width(element: HTMLElement): number {
  return parse_css_length_to_px(element.style.width)
    || Number(element.getAttribute("width") || 0)
    || element.scrollWidth
    || element.offsetWidth
    || element.getBoundingClientRect().width;
}

function parse_css_length_to_px(value: string): number {
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
