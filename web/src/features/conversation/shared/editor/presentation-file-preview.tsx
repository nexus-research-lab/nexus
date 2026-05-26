"use client";

import { useEffect, useRef, useState } from "react";
import { ChevronLeft, ChevronRight, Eye, FileText, FileWarning, LoaderCircle } from "lucide-react";
import type JSZip from "jszip";

import { get_workspace_file_preview_url } from "@/lib/api/agent-manage-api";
import { cn } from "@/lib/utils";
import { ConversationResizeHandle } from "./conversation-resize-handle";
import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
} from "./workspace-file-preview-chrome";

const MAX_PPTX_PREVIEW_BYTES = 15 * 1024 * 1024;
const EMU_PER_PIXEL = 9525;
const DEFAULT_SLIDE_WIDTH_EMU = 12192000;
const DEFAULT_SLIDE_HEIGHT_EMU = 6858000;
const RELATIONSHIP_NAMESPACE = "http://schemas.openxmlformats.org/officeDocument/2006/relationships";
const SLIDE_LAYOUT_RELATIONSHIP_TYPE = `${RELATIONSHIP_NAMESPACE}/slideLayout`;
const SLIDE_MASTER_RELATIONSHIP_TYPE = `${RELATIONSHIP_NAMESPACE}/slideMaster`;
const ROUND_RECT_RADIUS_RATIO = 0.08;
const ROUND_RECT_MAX_RADIUS = 14;
const MIN_DECORATION_SHAPE_SIZE = 28;
const MIN_BACKGROUND_LIKE_SHAPE_SIZE = 240;

const SCHEME_COLORS: Record<string, string> = {
  accent1: "#4472c4",
  accent2: "#ed7d31",
  accent3: "#a5a5a5",
  accent4: "#ffc000",
  accent5: "#5b9bd5",
  accent6: "#70ad47",
  bg1: "#ffffff",
  bg2: "#f2f2f2",
  dk1: "#111827",
  dk2: "#1f2937",
  lt1: "#ffffff",
  lt2: "#f8fafc",
  tx1: "#111827",
  tx2: "#374151",
};

type PresentationPreviewStatus =
  | { state: "loading"; message: string }
  | { state: "loaded"; slide_count: number }
  | { state: "error"; message: string };

type PresentationShapeGeometry =
  | "diamond"
  | "ellipse"
  | "line"
  | "rect"
  | "roundRect"
  | "triangle"
  | "unsupported";

interface PresentationTextRun {
  bold?: boolean;
  color?: string;
  font_face?: string;
  font_size: number;
  italic?: boolean;
  text: string;
}

interface PresentationParagraph {
  align?: "center" | "left" | "right";
  bullet?: string;
  bullet_indent: number;
  font_size: number;
  line_height: number;
  runs: PresentationTextRun[];
  text: string;
}

interface PresentationShapeElement {
  fill?: string;
  geometry: PresentationShapeGeometry;
  height: number;
  id: string;
  paragraphs: PresentationParagraph[];
  stroke?: string;
  stroke_width: number;
  text_anchor: "bottom" | "center" | "top";
  type: "shape";
  width: number;
  x: number;
  y: number;
}

interface PresentationImageElement {
  height: number;
  id: string;
  src: string;
  type: "image";
  width: number;
  x: number;
  y: number;
}

type PresentationElement = PresentationImageElement | PresentationShapeElement;

interface PresentationSlide {
  background: string;
  elements: PresentationElement[];
  height: number;
  id: string;
  title: string;
  width: number;
}

interface PresentationRelationship {
  target: string;
  target_mode?: string;
  type?: string;
}

interface PresentationParseResult {
  object_urls: string[];
  slides: PresentationSlide[];
}

type PresentationTransform = Pick<PresentationShapeElement, "height" | "width" | "x" | "y">;

interface PresentationPlaceholderStyle {
  fill?: string;
  geometry: PresentationShapeGeometry;
  key: string;
  stroke?: string;
  stroke_width: number;
  transform: PresentationTransform;
}

interface PresentationPart {
  background?: string;
  elements: PresentationElement[];
  placeholder_styles: Map<string, PresentationPlaceholderStyle>;
  rels: Record<string, PresentationRelationship>;
}

interface PresentationShapeTreeContext {
  element_index: number;
  fallback_placeholders?: Map<string, PresentationPlaceholderStyle>;
  id_prefix: string;
  include_placeholder_shapes: boolean;
}

interface PresentationShapeTreeResult {
  elements: PresentationElement[];
  placeholder_styles: Map<string, PresentationPlaceholderStyle>;
}

interface PresentationGroupTransform extends PresentationTransform {
  child_height: number;
  child_width: number;
  child_x: number;
  child_y: number;
}

interface PresentationFilePreviewProps {
  agent_id: string;
  embedded?: boolean;
  file_name: string;
  is_preview_focused?: boolean;
  on_resize_start: () => void;
  on_toggle_preview_focus?: () => void;
  path: string;
}

export function PresentationFilePreview({
  agent_id,
  embedded,
  file_name,
  is_preview_focused,
  on_resize_start,
  on_toggle_preview_focus,
  path,
}: PresentationFilePreviewProps) {
  const cleanup_urls_ref = useRef<() => void>(() => undefined);
  const [slides, set_slides] = useState<PresentationSlide[]>([]);
  const [active_slide_index, set_active_slide_index] = useState(0);
  const [status, set_status] = useState<PresentationPreviewStatus>({
    state: "loading",
    message: "加载演示文稿预览中",
  });

  useEffect(() => {
    const abort_controller = new AbortController();
    let cancelled = false;

    cleanup_urls_ref.current();
    cleanup_urls_ref.current = () => undefined;
    set_slides([]);
    set_active_slide_index(0);

    async function load_preview() {
      set_status({ state: "loading", message: "读取 pptx 文件中" });

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
        if (content_length && Number(content_length) > MAX_PPTX_PREVIEW_BYTES) {
          throw new Error("pptx 文件超过 15MB，建议下载后查看");
        }

        const buffer = await response.arrayBuffer();
        if (cancelled) {
          return;
        }
        if (buffer.byteLength > MAX_PPTX_PREVIEW_BYTES) {
          throw new Error("pptx 文件超过 15MB，建议下载后查看");
        }

        set_status({ state: "loading", message: "解析 pptx 文件中" });
        const result = await parse_pptx(buffer);
        if (cancelled) {
          revoke_object_urls(result.object_urls);
          return;
        }

        cleanup_urls_ref.current = () => revoke_object_urls(result.object_urls);
        set_slides(result.slides);
        set_active_slide_index(0);
        set_status({ state: "loaded", slide_count: result.slides.length });
      } catch (preview_error) {
        if (cancelled || abort_controller.signal.aborted) {
          return;
        }
        const message = preview_error instanceof Error ? preview_error.message : "pptx 预览失败";
        cleanup_urls_ref.current();
        cleanup_urls_ref.current = () => undefined;
        set_slides([]);
        set_status({ state: "error", message });
      }
    }

    void load_preview();

    return () => {
      cancelled = true;
      abort_controller.abort();
      cleanup_urls_ref.current();
      cleanup_urls_ref.current = () => undefined;
    };
  }, [agent_id, path]);

  const is_loaded = status.state === "loaded";
  const is_loading = status.state === "loading";
  const has_error = status.state === "error";
  const active_slide = slides[Math.min(active_slide_index, Math.max(slides.length - 1, 0))];

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
              pptx 预览
            </span>
            {has_error ? (
              <span className="flex items-center gap-1 text-destructive">
                <FileWarning className="h-3 w-3" />
                加载失败
              </span>
            ) : is_loaded ? (
              <span className="flex items-center gap-1 text-emerald-600">
                <Eye className="h-3 w-3" />
                已加载 {status.slide_count} 页
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

      <div className="min-h-0 flex-1 overflow-hidden bg-[var(--surface-panel-subtle-background)]">
        {has_error ? (
          <div className="flex h-full items-center justify-center p-8 text-center">
            <div className="max-w-sm">
              <FileWarning className="mx-auto h-12 w-12 text-(--icon-muted)" />
              <p className="mt-4 text-sm font-medium text-(--text-strong)">pptx 预览失败</p>
              <p className="mt-2 text-xs leading-5 text-(--text-soft)">{status.message}</p>
            </div>
          </div>
        ) : active_slide ? (
          <div className="flex h-full min-h-0">
            {slides.length > 1 ? (
              <aside className="soft-scrollbar hidden w-36 shrink-0 overflow-auto border-r divider-subtle bg-(--surface-panel-background) p-3 md:block">
                <div className="space-y-2">
                  {slides.map((slide, index) => (
                    <button
                      className={cn(
                        "w-full rounded-[6px] border p-1 text-left transition-colors",
                        index === active_slide_index
                          ? "border-primary/45 bg-primary/8"
                          : "border-(--divider-subtle-color) bg-(--surface-panel-subtle-background) hover:border-primary/30",
                      )}
                      key={slide.id}
                      onClick={() => set_active_slide_index(index)}
                      type="button"
                    >
                      <PresentationSlideCanvas class_name="rounded-[2px] shadow-none" slide={slide} thumbnail />
                      <span className="mt-1 block truncate text-[10px] font-medium text-(--text-muted)">
                        {index + 1}. {slide.title}
                      </span>
                    </button>
                  ))}
                </div>
              </aside>
            ) : null}

            <div className="soft-scrollbar min-h-0 flex-1 overflow-auto p-5">
              <div className="mx-auto flex w-full max-w-6xl flex-col gap-3">
                <div className="flex items-center justify-between gap-3 text-xs text-(--text-muted)">
                  <span className="min-w-0 truncate">
                    {active_slide_index + 1} / {slides.length} · {active_slide.title}
                  </span>
                  {slides.length > 1 ? (
                    <div className="flex shrink-0 items-center gap-1">
                      <button
                        aria-label="上一页幻灯片"
                        className="inline-flex h-8 w-8 items-center justify-center rounded-[6px] border border-(--divider-subtle-color) bg-(--surface-panel-background) text-(--text-default) transition-colors hover:border-primary/30 hover:text-primary disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
                        disabled={active_slide_index <= 0}
                        onClick={() => set_active_slide_index((index) => Math.max(index - 1, 0))}
                        type="button"
                      >
                        <ChevronLeft className="h-4 w-4" />
                      </button>
                      <button
                        aria-label="下一页幻灯片"
                        className="inline-flex h-8 w-8 items-center justify-center rounded-[6px] border border-(--divider-subtle-color) bg-(--surface-panel-background) text-(--text-default) transition-colors hover:border-primary/30 hover:text-primary disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
                        disabled={active_slide_index >= slides.length - 1}
                        onClick={() => set_active_slide_index((index) => Math.min(index + 1, slides.length - 1))}
                        type="button"
                      >
                        <ChevronRight className="h-4 w-4" />
                      </button>
                    </div>
                  ) : null}
                </div>
                <PresentationSlideCanvas slide={active_slide} />
              </div>
            </div>
          </div>
        ) : (
          <div className="flex h-full items-center justify-center p-8 text-center">
            <div className="max-w-xs">
              <LoaderCircle className="mx-auto h-8 w-8 animate-spin text-primary" />
              <p className="mt-3 text-sm font-medium text-(--text-strong)">
                {is_loading ? status.message : "正在加载 pptx 预览"}
              </p>
            </div>
          </div>
        )}
      </div>
    </>
  );
}

function PresentationSlideCanvas({
  class_name,
  slide,
  thumbnail = false,
}: {
  class_name?: string;
  slide: PresentationSlide;
  thumbnail?: boolean;
}) {
  return (
    <svg
      aria-label={slide.title}
      className={cn(
        "block w-full bg-white shadow-[0_18px_42px_rgba(15,23,42,0.16)]",
        thumbnail ? "rounded-[2px] shadow-sm" : "rounded-[2px]",
        class_name,
      )}
      role="img"
      style={{ aspectRatio: `${slide.width} / ${slide.height}` }}
      viewBox={`0 0 ${slide.width} ${slide.height}`}
    >
      <rect fill={slide.background} height={slide.height} width={slide.width} x={0} y={0} />
      {slide.elements.map((element) => {
        if (element.type === "image") {
          return (
            <image
              height={element.height}
              href={element.src}
              key={element.id}
              preserveAspectRatio="xMidYMid meet"
              width={element.width}
              x={element.x}
              y={element.y}
            />
          );
        }

        return <PresentationShape key={element.id} shape={element} thumbnail={thumbnail} />;
      })}
    </svg>
  );
}

function PresentationShape({
  shape,
  thumbnail,
}: {
  shape: PresentationShapeElement;
  thumbnail: boolean;
}) {
  const stroke = shape.stroke || "none";
  const fill = shape.geometry === "line" ? "none" : shape.fill || "transparent";
  const text_padding = thumbnail
    ? Math.max(Math.min(shape.width, shape.height) * 0.03, 4)
    : Math.max(Math.min(shape.width, shape.height) * 0.045, 6);
  const justify_content = shape.text_anchor === "center"
    ? "center"
    : shape.text_anchor === "bottom"
      ? "flex-end"
      : "flex-start";

  return (
    <g>
      {render_shape_geometry(shape, fill, stroke)}
      {shape.paragraphs.length > 0 ? (
        <foreignObject height={shape.height} width={shape.width} x={shape.x} y={shape.y}>
          <div
            style={{
              boxSizing: "border-box",
              display: "flex",
              flexDirection: "column",
              height: "100%",
              justifyContent: justify_content,
              overflow: "hidden",
              padding: text_padding,
              width: "100%",
            }}
          >
            {shape.paragraphs.map((paragraph, index) => (
              <p
                key={`${shape.id}-paragraph-${index}`}
                style={{
                  columnGap: paragraph.bullet ? paragraph.font_size * 0.45 : undefined,
                  display: paragraph.bullet ? "grid" : "block",
                  fontSize: paragraph.font_size,
                  gridTemplateColumns: paragraph.bullet ? `${paragraph.bullet_indent}px minmax(0, 1fr)` : undefined,
                  lineHeight: paragraph.line_height,
                  margin: index === 0 ? 0 : `${paragraph.font_size * 0.42}px 0 0`,
                  textAlign: paragraph.align || "left",
                  whiteSpace: "normal",
                  wordBreak: paragraph.align === "center" ? "keep-all" : "normal",
                }}
              >
                {paragraph.bullet ? (
                  <span
                    style={{
                      color: paragraph.runs[0]?.color || "#111827",
                      fontFamily: "Arial, sans-serif",
                      fontSize: paragraph.font_size,
                      fontWeight: 700,
                      lineHeight: paragraph.line_height,
                    }}
                  >
                    {paragraph.bullet}
                  </span>
                ) : null}
                <span style={{ minWidth: 0, overflowWrap: paragraph.align === "center" ? "normal" : "break-word" }}>
                  {paragraph.runs.map((run, run_index) => (
                    <span
                      key={`${shape.id}-paragraph-${index}-run-${run_index}`}
                      style={{
                        color: run.color || "#111827",
                        fontFamily: run.font_face || "Arial, sans-serif",
                        fontSize: run.font_size,
                        fontStyle: run.italic ? "italic" : "normal",
                        fontWeight: run.bold ? 700 : 400,
                      }}
                    >
                      {run.text}
                    </span>
                  ))}
                </span>
              </p>
            ))}
          </div>
        </foreignObject>
      ) : null}
    </g>
  );
}

function render_shape_geometry(shape: PresentationShapeElement, fill: string, stroke: string) {
  if (shape.geometry === "unsupported") {
    return null;
  }

  const common_props = {
    fill,
    stroke,
    strokeWidth: shape.stroke === undefined ? 0 : shape.stroke_width,
  };

  switch (shape.geometry) {
    case "diamond":
      return (
        <polygon
          points={[
            `${shape.x + shape.width / 2},${shape.y}`,
            `${shape.x + shape.width},${shape.y + shape.height / 2}`,
            `${shape.x + shape.width / 2},${shape.y + shape.height}`,
            `${shape.x},${shape.y + shape.height / 2}`,
          ].join(" ")}
          {...common_props}
        />
      );
    case "ellipse":
      return (
        <ellipse
          cx={shape.x + shape.width / 2}
          cy={shape.y + shape.height / 2}
          rx={Math.abs(shape.width / 2)}
          ry={Math.abs(shape.height / 2)}
          {...common_props}
        />
      );
    case "line":
      return (
        <line
          stroke={stroke === "none" ? "#64748b" : stroke}
          strokeWidth={Math.max(shape.stroke_width, 1)}
          x1={shape.x}
          x2={shape.x + shape.width}
          y1={shape.y}
          y2={shape.y + shape.height}
        />
      );
    case "roundRect": {
      const radius = Math.min(
        Math.min(shape.width, shape.height) * ROUND_RECT_RADIUS_RATIO,
        ROUND_RECT_MAX_RADIUS,
      );
      return (
        <rect
          height={shape.height}
          rx={radius}
          ry={radius}
          width={shape.width}
          x={shape.x}
          y={shape.y}
          {...common_props}
        />
      );
    }
    case "triangle":
      return (
        <polygon
          points={[
            `${shape.x + shape.width / 2},${shape.y}`,
            `${shape.x + shape.width},${shape.y + shape.height}`,
            `${shape.x},${shape.y + shape.height}`,
          ].join(" ")}
          {...common_props}
        />
      );
    case "rect":
      return (
        <rect
          height={shape.height}
          width={shape.width}
          x={shape.x}
          y={shape.y}
          {...common_props}
        />
      );
  }
}

async function parse_pptx(buffer: ArrayBuffer): Promise<PresentationParseResult> {
  const { default: JSZipConstructor } = await import("jszip");
  const zip = await JSZipConstructor.loadAsync(buffer);
  const object_urls: string[] = [];

  try {
    const presentation_xml = await read_zip_text(zip, "ppt/presentation.xml");
    const presentation_doc = parse_xml(presentation_xml);
    const presentation_rels = await read_relationships(zip, "ppt/presentation.xml");
    const { height, width } = read_slide_size(presentation_doc);
    const slide_paths = read_slide_paths(presentation_doc, presentation_rels);
    const resolved_slide_paths = slide_paths.length > 0 ? slide_paths : fallback_slide_paths(zip);

    if (resolved_slide_paths.length === 0) {
      throw new Error("pptx 文件中没有可预览的幻灯片");
    }

    const slides: PresentationSlide[] = [];
    for (let index = 0; index < resolved_slide_paths.length; index += 1) {
      const slide = await parse_slide(zip, resolved_slide_paths[index], index, width, height, object_urls);
      slides.push(slide);
    }

    return { object_urls, slides };
  } catch (error) {
    revoke_object_urls(object_urls);
    throw error;
  }
}

function read_slide_size(presentation_doc: Document): { height: number; width: number } {
  const slide_size = first_descendant_by_local_name(presentation_doc, "sldSz");
  const width_emu = Number(slide_size?.getAttribute("cx") || DEFAULT_SLIDE_WIDTH_EMU);
  const height_emu = Number(slide_size?.getAttribute("cy") || DEFAULT_SLIDE_HEIGHT_EMU);
  return {
    height: Math.max(emu_to_pixel(height_emu), 1),
    width: Math.max(emu_to_pixel(width_emu), 1),
  };
}

function read_slide_paths(
  presentation_doc: Document,
  presentation_rels: Record<string, PresentationRelationship>,
): string[] {
  return descendants_by_local_name(presentation_doc, "sldId")
    .map((slide_id) => {
      const rel_id = relationship_attribute(slide_id, "id");
      const rel = rel_id ? presentation_rels[rel_id] : undefined;
      return rel ? resolve_relationship_target("ppt/presentation.xml", rel.target) : null;
    })
    .filter((path): path is string => !!path);
}

function fallback_slide_paths(zip: JSZip): string[] {
  return Object.keys(zip.files)
    .filter((path) => /^ppt\/slides\/slide\d+\.xml$/i.test(path))
    .sort((left, right) => {
      const left_number = Number(left.match(/slide(\d+)\.xml$/i)?.[1] || 0);
      const right_number = Number(right.match(/slide(\d+)\.xml$/i)?.[1] || 0);
      return left_number - right_number;
    });
}

async function parse_slide(
  zip: JSZip,
  slide_path: string,
  index: number,
  width: number,
  height: number,
  object_urls: string[],
): Promise<PresentationSlide> {
  const slide_xml = await read_zip_text(zip, slide_path);
  const slide_doc = parse_xml(slide_xml);
  const slide_rels = await read_relationships(zip, slide_path);
  const layout_path = resolve_related_part_path(slide_path, slide_rels, SLIDE_LAYOUT_RELATIONSHIP_TYPE);
  const layout_rels = layout_path ? await read_relationships(zip, layout_path) : {};
  const master_path = layout_path
    ? resolve_related_part_path(layout_path, layout_rels, SLIDE_MASTER_RELATIONSHIP_TYPE)
    : null;
  const master_part = master_path ? await parse_presentation_part(zip, master_path, object_urls) : null;
  const layout_part = layout_path
    ? await parse_presentation_part(zip, layout_path, object_urls, master_part?.placeholder_styles)
    : null;
  const inherited_placeholders = merge_placeholder_styles(
    master_part?.placeholder_styles,
    layout_part?.placeholder_styles,
  );
  const background = read_slide_background(slide_doc)
    || layout_part?.background
    || master_part?.background
    || "#ffffff";
  const shape_tree = first_descendant_by_local_name(slide_doc, "spTree");
  const slide_result = shape_tree ? await parse_shape_tree(
    zip,
    slide_path,
    slide_rels,
    shape_tree,
    object_urls,
    {
      element_index: 0,
      fallback_placeholders: inherited_placeholders,
      id_prefix: `slide-${index + 1}`,
      include_placeholder_shapes: true,
    },
  ) : { elements: [], placeholder_styles: new Map<string, PresentationPlaceholderStyle>() };
  const elements = [
    ...(master_part?.elements ?? []),
    ...(layout_part?.elements ?? []),
    ...slide_result.elements,
  ];
  const first_text = slide_result.elements
    .flatMap((element) => element.type === "shape" ? element.paragraphs : [])
    .map((paragraph) => paragraph.text.trim())
    .find(Boolean);

  return {
    background,
    elements,
    height,
    id: `slide-${index + 1}`,
    title: first_text || `幻灯片 ${index + 1}`,
    width,
  };
}

async function parse_presentation_part(
  zip: JSZip,
  part_path: string,
  object_urls: string[],
  fallback_placeholders?: Map<string, PresentationPlaceholderStyle>,
): Promise<PresentationPart | null> {
  if (!zip.file(part_path)) {
    return null;
  }

  const part_xml = await read_zip_text(zip, part_path);
  const part_doc = parse_xml(part_xml);
  const rels = await read_relationships(zip, part_path);
  const shape_tree = first_descendant_by_local_name(part_doc, "spTree");
  const result = shape_tree ? await parse_shape_tree(zip, part_path, rels, shape_tree, object_urls, {
    element_index: 0,
    fallback_placeholders,
    id_prefix: part_path.replace(/[^a-z0-9]+/gi, "-"),
    include_placeholder_shapes: false,
  }) : { elements: [], placeholder_styles: new Map<string, PresentationPlaceholderStyle>() };

  return {
    background: read_slide_background(part_doc),
    elements: result.elements,
    placeholder_styles: result.placeholder_styles,
    rels,
  };
}

function resolve_related_part_path(
  source_path: string,
  source_rels: Record<string, PresentationRelationship>,
  relationship_type: string,
): string | null {
  const rel = Object.values(source_rels).find((relationship) => relationship.type === relationship_type);
  if (!rel || rel.target_mode === "External") {
    return null;
  }
  return resolve_relationship_target(source_path, rel.target);
}

function merge_placeholder_styles(
  base?: Map<string, PresentationPlaceholderStyle>,
  override?: Map<string, PresentationPlaceholderStyle>,
): Map<string, PresentationPlaceholderStyle> {
  return new Map([
    ...(base?.entries() ?? []),
    ...(override?.entries() ?? []),
  ]);
}

async function parse_shape_tree(
  zip: JSZip,
  slide_path: string,
  rels: Record<string, PresentationRelationship>,
  shape_tree: Element,
  object_urls: string[],
  context: PresentationShapeTreeContext,
  group_transform?: PresentationGroupTransform | null,
): Promise<PresentationShapeTreeResult> {
  const elements: PresentationElement[] = [];
  const placeholder_styles = new Map<string, PresentationPlaceholderStyle>();
  const children = Array.from(shape_tree.children);

  for (const child of children) {
    switch (child.localName) {
      case "cxnSp":
      case "sp": {
        const parsed_shape = parse_shape(child, `${context.id_prefix}-shape-${context.element_index}`, context);
        context.element_index += 1;
        if (parsed_shape.placeholder_style) {
          placeholder_styles.set(parsed_shape.placeholder_style.key, parsed_shape.placeholder_style);
        }
        if (parsed_shape.shape && (!parsed_shape.is_placeholder || context.include_placeholder_shapes)) {
          elements.push(parsed_shape.shape);
        }
        break;
      }
      case "grpSp": {
        const group_result = await parse_shape_tree(
          zip,
          slide_path,
          rels,
          child,
          object_urls,
          context,
          read_group_transform(child),
        );
        group_result.placeholder_styles.forEach((style, key) => {
          placeholder_styles.set(key, style);
        });
        elements.push(...group_result.elements);
        break;
      }
      case "pic": {
        const image = await parse_picture(
          zip,
          slide_path,
          rels,
          child,
          `${context.id_prefix}-image-${context.element_index}`,
          object_urls,
        );
        context.element_index += 1;
        if (image) {
          elements.push(image);
        }
        break;
      }
      default:
        break;
    }
  }

  if (!group_transform) {
    return { elements, placeholder_styles };
  }

  return {
    elements: elements.map((element) => apply_group_transform_to_element(element, group_transform)),
    placeholder_styles: map_group_placeholder_styles(placeholder_styles, group_transform),
  };
}

function parse_shape(
  element: Element,
  id: string,
  context: PresentationShapeTreeContext,
): {
  is_placeholder: boolean;
  placeholder_style: PresentationPlaceholderStyle | null;
  shape: PresentationShapeElement | null;
} {
  const shape_properties = first_child_by_local_name(element, "spPr");
  const placeholder_key = read_placeholder_key(element);
  const fallback_placeholder = placeholder_key ? context.fallback_placeholders?.get(placeholder_key) : undefined;
  const transform = read_transform(shape_properties) || fallback_placeholder?.transform || null;
  if (!transform) {
    return {
      is_placeholder: !!placeholder_key,
      placeholder_style: null,
      shape: null,
    };
  }

  const text_body = first_child_by_local_name(element, "txBody");
  const paragraphs = parse_text_body(text_body, transform.width);
  const text_anchor = read_text_anchor(first_child_by_local_name(text_body, "bodyPr"));
  const fill = read_fill_color(shape_properties) || fallback_placeholder?.fill;
  const stroke = read_stroke_color(shape_properties) || fallback_placeholder?.stroke;
  const stroke_width = read_stroke_width(shape_properties) || fallback_placeholder?.stroke_width || 1;
  const geometry = read_shape_geometry(shape_properties, element.localName === "cxnSp", fallback_placeholder?.geometry);
  const placeholder_style = placeholder_key ? {
    fill,
    geometry,
    key: placeholder_key,
    stroke,
    stroke_width,
    transform,
  } : null;

  if (should_skip_shape_preview({ fill, geometry, height: transform.height, paragraphs, stroke, width: transform.width })) {
    return {
      is_placeholder: !!placeholder_key,
      placeholder_style,
      shape: null,
    };
  }

  return {
    is_placeholder: !!placeholder_key,
    placeholder_style,
    shape: {
      ...transform,
      fill,
      geometry,
      id,
      paragraphs,
      stroke,
      stroke_width,
      text_anchor,
      type: "shape",
    },
  };
}

function should_skip_shape_preview({
  fill,
  geometry,
  height,
  paragraphs,
  stroke,
  width,
}: {
  fill?: string;
  geometry: PresentationShapeGeometry;
  height: number;
  paragraphs: PresentationParagraph[];
  stroke?: string;
  width: number;
}): boolean {
  if (geometry === "line") {
    return false;
  }
  if (geometry === "unsupported" && paragraphs.length === 0) {
    return true;
  }
  if (!fill && !stroke && paragraphs.length === 0) {
    return true;
  }

  // 中文注释：PPT 里有些装饰点/图标会以复杂几何降级成小描边矩形。
  // 预览无法高保真还原时，隐藏它比显示误导性的半成品更接近系统预览体验。
  return (
    geometry === "rect" &&
    !fill &&
    !!stroke &&
    paragraphs.length === 0 &&
    Math.min(width, height) <= MIN_DECORATION_SHAPE_SIZE
  ) || (
    geometry === "roundRect" &&
    is_plain_white_fill(fill) &&
    !stroke &&
    paragraphs.length === 0 &&
    Math.min(width, height) >= MIN_BACKGROUND_LIKE_SHAPE_SIZE
  );
}

function is_plain_white_fill(fill?: string): boolean {
  const normalized_fill = fill?.toLowerCase();
  return normalized_fill === "#ffffff" || normalized_fill === "#fff";
}

async function parse_picture(
  zip: JSZip,
  slide_path: string,
  rels: Record<string, PresentationRelationship>,
  element: Element,
  id: string,
  object_urls: string[],
): Promise<PresentationImageElement | null> {
  const shape_properties = first_child_by_local_name(element, "spPr");
  const transform = read_transform(shape_properties);
  const blip = first_descendant_by_local_name(element, "blip");
  const rel_id = blip ? relationship_attribute(blip, "embed") || relationship_attribute(blip, "link") : undefined;
  const rel = rel_id ? rels[rel_id] : undefined;

  if (!transform || !rel || rel.target_mode === "External") {
    return null;
  }

  const media_path = resolve_relationship_target(slide_path, rel.target);
  const media_file = zip.file(media_path);
  if (!media_file) {
    return null;
  }

  const blob = await media_file.async("blob");
  const src = URL.createObjectURL(blob);
  object_urls.push(src);

  return {
    ...transform,
    id,
    src,
    type: "image",
  };
}

function parse_text_body(text_body: Element | null, shape_width: number): PresentationParagraph[] {
  if (!text_body) {
    return [];
  }

  const list_style = first_child_by_local_name(text_body, "lstStyle");

  return children_by_local_name(text_body, "p")
    .map((paragraph) => {
      const paragraph_properties = first_child_by_local_name(paragraph, "pPr");
      const list_paragraph_properties = read_list_paragraph_properties(list_style, paragraph_properties);
      const default_run_properties = [
        first_child_by_local_name(paragraph_properties, "defRPr"),
        first_child_by_local_name(list_paragraph_properties, "defRPr"),
        first_child_by_local_name(paragraph, "endParaRPr"),
      ];
      const align = read_paragraph_align(paragraph_properties);
      const default_font_size = read_font_size_from_candidates(default_run_properties, shape_width);
      const runs = children_by_local_name(paragraph, "r")
        .map((run) => parse_text_run(run, default_run_properties, shape_width, default_font_size))
        .filter((run): run is PresentationTextRun => !!run && run.text.length > 0);
      const fallback_text = runs.length === 0 ? first_descendant_by_local_name(paragraph, "t")?.textContent || "" : "";
      const text_runs = runs.length > 0
        ? runs
        : [{
          color: read_fill_color_from_candidates(default_run_properties) || "#111827",
          font_face: read_font_face_from_candidates(default_run_properties),
          font_size: default_font_size,
          text: fallback_text,
        }];
      const text = text_runs.map((run) => run.text).join("");
      const paragraph_font_size = text_runs[0]?.font_size || default_font_size;

      return {
        align,
        bullet: read_paragraph_bullet(paragraph_properties),
        bullet_indent: read_paragraph_bullet_indent(paragraph_properties, paragraph_font_size),
        font_size: paragraph_font_size,
        line_height: read_paragraph_line_height(paragraph_properties, list_paragraph_properties),
        runs: text_runs,
        text,
      };
    })
    .filter((paragraph) => paragraph.text.trim().length > 0);
}

function parse_text_run(
  run: Element,
  default_run_properties: Array<Element | null>,
  shape_width: number,
  default_font_size: number,
): PresentationTextRun | null {
  const text = first_descendant_by_local_name(run, "t")?.textContent || "";
  if (!text) {
    return null;
  }

  const run_properties = first_child_by_local_name(run, "rPr");
  const run_property_chain = [run_properties, ...default_run_properties];
  return {
    bold: read_boolean_attribute_from_candidates(run_property_chain, "b", false),
    color: read_fill_color_from_candidates(run_property_chain) || "#111827",
    font_face: read_font_face_from_candidates(run_property_chain),
    font_size: read_font_size_from_candidates(run_property_chain, shape_width, default_font_size),
    italic: read_boolean_attribute_from_candidates(run_property_chain, "i", false),
    text,
  };
}

function read_list_paragraph_properties(list_style: Element | null, paragraph_properties: Element | null): Element | null {
  const level = Math.max(Number(paragraph_properties?.getAttribute("lvl") || 0), 0);
  return first_child_by_local_name(list_style, `lvl${level + 1}pPr`)
    || first_child_by_local_name(list_style, "defPPr");
}

function read_boolean_attribute_from_candidates(
  elements: Array<Element | null>,
  attribute: string,
  fallback: boolean,
): boolean {
  for (const element of elements) {
    const value = element?.getAttribute(attribute);
    if (value === "1" || value === "true") {
      return true;
    }
    if (value === "0" || value === "false") {
      return false;
    }
  }
  return fallback;
}

function read_font_face(run_properties: Element | null): string | undefined {
  return first_descendant_by_local_name(run_properties, "ea")?.getAttribute("typeface")
    || first_descendant_by_local_name(run_properties, "latin")?.getAttribute("typeface")
    || undefined;
}

function read_font_face_from_candidates(elements: Array<Element | null>): string | undefined {
  for (const element of elements) {
    const font_face = read_font_face(element);
    if (font_face) {
      return font_face;
    }
  }
  return undefined;
}

function read_fill_color_from_candidates(elements: Array<Element | null>): string | undefined {
  for (const element of elements) {
    const color = read_fill_color(element);
    if (color) {
      return color;
    }
  }
  return undefined;
}

function read_paragraph_bullet(paragraph_properties: Element | null): string | undefined {
  if (!paragraph_properties || first_child_by_local_name(paragraph_properties, "buNone")) {
    return undefined;
  }
  return first_child_by_local_name(paragraph_properties, "buChar")?.getAttribute("char") || undefined;
}

function read_paragraph_bullet_indent(paragraph_properties: Element | null, font_size: number): number {
  const margin = Number(paragraph_properties?.getAttribute("marL") || 0);
  if (margin > 0) {
    return Math.max(emu_to_pixel(margin), font_size * 1.2);
  }
  return font_size * 1.35;
}

function read_paragraph_line_height(
  paragraph_properties: Element | null,
  list_paragraph_properties?: Element | null,
): number {
  const line_spacing = first_descendant_by_local_name(paragraph_properties, "lnSpc")
    || first_descendant_by_local_name(list_paragraph_properties || null, "lnSpc");
  const spacing_percent = Number(first_child_by_local_name(line_spacing, "spcPct")?.getAttribute("val") || 0);
  if (spacing_percent > 0) {
    return Math.max(spacing_percent / 100000, 1);
  }
  return 1.18;
}

function read_placeholder_key(element: Element): string | undefined {
  const placeholder = first_descendant_by_local_name(element, "ph");
  if (!placeholder) {
    return undefined;
  }

  const index = placeholder.getAttribute("idx");
  if (index) {
    return `idx:${index}`;
  }

  const type = placeholder.getAttribute("type") || "body";
  return `type:${type}`;
}

function read_text_anchor(body_properties: Element | null): PresentationShapeElement["text_anchor"] {
  const anchor = body_properties?.getAttribute("anchor");
  if (anchor === "ctr") {
    return "center";
  }
  if (anchor === "b") {
    return "bottom";
  }
  return "top";
}

function read_group_transform(element: Element): PresentationGroupTransform | null {
  const group_properties = first_child_by_local_name(element, "grpSpPr");
  const transform = first_child_by_local_name(group_properties, "xfrm");
  const offset = first_child_by_local_name(transform, "off");
  const extent = first_child_by_local_name(transform, "ext");
  const child_offset = first_child_by_local_name(transform, "chOff");
  const child_extent = first_child_by_local_name(transform, "chExt");
  if (!offset || !extent || !child_offset || !child_extent) {
    return null;
  }

  const child_width = emu_to_pixel(Number(child_extent.getAttribute("cx") || 0));
  const child_height = emu_to_pixel(Number(child_extent.getAttribute("cy") || 0));
  const width = emu_to_pixel(Number(extent.getAttribute("cx") || 0));
  const height = emu_to_pixel(Number(extent.getAttribute("cy") || 0));
  if (child_width <= 0 || child_height <= 0 || width <= 0 || height <= 0) {
    return null;
  }

  return {
    child_height,
    child_width,
    child_x: emu_to_pixel(Number(child_offset.getAttribute("x") || 0)),
    child_y: emu_to_pixel(Number(child_offset.getAttribute("y") || 0)),
    height,
    width,
    x: emu_to_pixel(Number(offset.getAttribute("x") || 0)),
    y: emu_to_pixel(Number(offset.getAttribute("y") || 0)),
  };
}

function apply_group_transform_to_element(
  element: PresentationElement,
  group_transform: PresentationGroupTransform,
): PresentationElement {
  const transform = apply_group_transform_to_rect(element, group_transform);
  if (element.type === "image") {
    return {
      ...element,
      ...transform,
    };
  }

  const scale = group_scale(group_transform);
  return {
    ...element,
    ...transform,
    paragraphs: element.paragraphs.map((paragraph) => ({
      ...paragraph,
      bullet_indent: paragraph.bullet_indent * scale,
      font_size: paragraph.font_size * scale,
      runs: paragraph.runs.map((run) => ({
        ...run,
        font_size: run.font_size * scale,
      })),
    })),
    stroke_width: element.stroke_width * scale,
  };
}

function map_group_placeholder_styles(
  placeholder_styles: Map<string, PresentationPlaceholderStyle>,
  group_transform: PresentationGroupTransform,
): Map<string, PresentationPlaceholderStyle> {
  return new Map(Array.from(placeholder_styles.entries()).map(([key, style]) => {
    const scale = group_scale(group_transform);
    return [key, {
      ...style,
      stroke_width: style.stroke_width * scale,
      transform: apply_group_transform_to_rect(style.transform, group_transform),
    }];
  }));
}

function apply_group_transform_to_rect(
  transform: PresentationTransform,
  group_transform: PresentationGroupTransform,
): PresentationTransform {
  const scale_x = group_transform.width / group_transform.child_width;
  const scale_y = group_transform.height / group_transform.child_height;
  return {
    height: transform.height * scale_y,
    width: transform.width * scale_x,
    x: group_transform.x + ((transform.x - group_transform.child_x) * scale_x),
    y: group_transform.y + ((transform.y - group_transform.child_y) * scale_y),
  };
}

function group_scale(group_transform: PresentationGroupTransform): number {
  return Math.min(
    group_transform.width / group_transform.child_width,
    group_transform.height / group_transform.child_height,
  );
}

function read_transform(shape_properties: Element | null): PresentationTransform | null {
  const transform = first_child_by_local_name(shape_properties, "xfrm") || first_descendant_by_local_name(shape_properties, "xfrm");
  const offset = first_child_by_local_name(transform, "off");
  const extent = first_child_by_local_name(transform, "ext");
  if (!offset || !extent) {
    return null;
  }

  return {
    height: emu_to_pixel(Number(extent.getAttribute("cy") || 0)),
    width: emu_to_pixel(Number(extent.getAttribute("cx") || 0)),
    x: emu_to_pixel(Number(offset.getAttribute("x") || 0)),
    y: emu_to_pixel(Number(offset.getAttribute("y") || 0)),
  };
}

function read_shape_geometry(
  shape_properties: Element | null,
  is_connector: boolean,
  fallback_geometry?: PresentationShapeGeometry,
): PresentationShapeGeometry {
  if (is_connector) {
    return "line";
  }

  const preset_geometry = first_child_by_local_name(shape_properties, "prstGeom");
  const preset = preset_geometry?.getAttribute("prst");
  switch (preset) {
    case "diamond":
      return "diamond";
    case "ellipse":
      return "ellipse";
    case "line":
      return "line";
    case "rect":
      return "rect";
    case "roundRect":
      return "roundRect";
    case "triangle":
    case "rtTriangle":
      return "triangle";
    default:
      return fallback_geometry || "unsupported";
  }
}

function read_slide_background(slide_doc: Document): string | undefined {
  const background = first_descendant_by_local_name(slide_doc, "bgPr");
  return read_fill_color(background);
}

function read_fill_color(element: Element | null): string | undefined {
  if (!element || first_child_by_local_name(element, "noFill")) {
    return undefined;
  }

  const solid_fill = first_descendant_by_local_name(element, "solidFill");
  if (!solid_fill) {
    return undefined;
  }

  const srgb_color = first_child_by_local_name(solid_fill, "srgbClr");
  const srgb_value = srgb_color?.getAttribute("val");
  if (srgb_value) {
    return apply_color_luminance(`#${srgb_value}`, srgb_color);
  }

  const system_color = first_child_by_local_name(solid_fill, "sysClr");
  const system_value = system_color?.getAttribute("lastClr");
  if (system_value) {
    return `#${system_value}`;
  }

  const preset_color = first_child_by_local_name(solid_fill, "prstClr");
  const preset_value = preset_color?.getAttribute("val");
  if (preset_value === "white") {
    return "#ffffff";
  }
  if (preset_value === "black") {
    return "#000000";
  }

  const scheme_color = first_child_by_local_name(solid_fill, "schemeClr");
  const scheme_value = scheme_color?.getAttribute("val");
  return scheme_value ? apply_color_luminance(SCHEME_COLORS[scheme_value], scheme_color) : undefined;
}

function apply_color_luminance(color: string | undefined, color_element: Element | null): string | undefined {
  if (!color) {
    return undefined;
  }

  const lum_mod = Number(first_child_by_local_name(color_element, "lumMod")?.getAttribute("val") || 100000);
  const lum_off = Number(first_child_by_local_name(color_element, "lumOff")?.getAttribute("val") || 0);
  if (lum_mod === 100000 && lum_off === 0) {
    return color;
  }

  const rgb = parse_hex_color(color);
  if (!rgb) {
    return color;
  }

  const channels = rgb.map((channel) => clamp_color_channel(
    (channel * lum_mod / 100000) + (255 * lum_off / 100000),
  ));
  return `#${channels.map((channel) => channel.toString(16).padStart(2, "0")).join("")}`;
}

function parse_hex_color(color: string): [number, number, number] | null {
  const normalized = color.replace("#", "");
  if (normalized.length !== 6) {
    return null;
  }

  return [
    Number.parseInt(normalized.slice(0, 2), 16),
    Number.parseInt(normalized.slice(2, 4), 16),
    Number.parseInt(normalized.slice(4, 6), 16),
  ];
}

function clamp_color_channel(value: number): number {
  return Math.max(0, Math.min(255, Math.round(value)));
}

function read_stroke_color(shape_properties: Element | null): string | undefined {
  const line = first_child_by_local_name(shape_properties, "ln");
  if (!line || first_child_by_local_name(line, "noFill")) {
    return undefined;
  }
  return read_fill_color(line) || "#64748b";
}

function read_stroke_width(shape_properties: Element | null): number {
  const line = first_child_by_local_name(shape_properties, "ln");
  const width = Number(line?.getAttribute("w") || 0);
  return width > 0 ? Math.max(emu_to_pixel(width), 1) : 1;
}

function read_paragraph_align(paragraph_properties: Element | null): PresentationParagraph["align"] {
  const align = paragraph_properties?.getAttribute("algn");
  if (align === "ctr") {
    return "center";
  }
  if (align === "r") {
    return "right";
  }
  return "left";
}

function read_font_size_from_candidates(
  elements: Array<Element | null>,
  shape_width: number,
  fallback_size?: number,
): number {
  for (const element of elements) {
    const size = Number(element?.getAttribute("sz") || 0);
    if (size > 0) {
      return Math.max((size / 100) * (96 / 72), 8);
    }
  }
  if (fallback_size) {
    return fallback_size;
  }
  return Math.max(Math.min(shape_width / 16, 24), 13);
}

async function read_relationships(zip: JSZip, part_path: string): Promise<Record<string, PresentationRelationship>> {
  const rels_path = relationship_part_path(part_path);
  const rels_file = zip.file(rels_path);
  if (!rels_file) {
    return {};
  }

  const rels_doc = parse_xml(await rels_file.async("text"));
  const relationships: Record<string, PresentationRelationship> = {};

  descendants_by_local_name(rels_doc, "Relationship").forEach((relationship) => {
    const id = relationship.getAttribute("Id");
    const target = relationship.getAttribute("Target");
    if (!id || !target) {
      return;
    }

    relationships[id] = {
      target,
      target_mode: relationship.getAttribute("TargetMode") || undefined,
      type: relationship.getAttribute("Type") || undefined,
    };
  });

  return relationships;
}

async function read_zip_text(zip: JSZip, file_path: string): Promise<string> {
  const file = zip.file(file_path);
  if (!file) {
    throw new Error(`pptx 缺少 ${file_path}`);
  }
  return file.async("text");
}

function parse_xml(xml: string): Document {
  const doc = new DOMParser().parseFromString(xml, "application/xml");
  const parse_error = first_descendant_by_local_name(doc, "parsererror");
  if (parse_error) {
    throw new Error("pptx XML 解析失败");
  }
  return doc;
}

function relationship_attribute(element: Element, local_name: string): string | undefined {
  return Array.from(element.attributes)
    .find((attribute) => attribute.localName === local_name && attribute.namespaceURI === RELATIONSHIP_NAMESPACE)
    ?.value;
}

function relationship_part_path(part_path: string): string {
  const normalized_path = normalize_zip_path(part_path);
  const parts = normalized_path.split("/");
  const file_name = parts.pop();
  return normalize_zip_path(`${parts.join("/")}/_rels/${file_name}.rels`);
}

function resolve_relationship_target(source_path: string, target: string): string {
  if (target.startsWith("/")) {
    return normalize_zip_path(target);
  }

  const source_parts = normalize_zip_path(source_path).split("/");
  source_parts.pop();
  return normalize_zip_path(`${source_parts.join("/")}/${target}`);
}

function normalize_zip_path(file_path: string): string {
  const segments: string[] = [];
  file_path.replace(/\\/g, "/").split("/").forEach((segment) => {
    if (!segment || segment === ".") {
      return;
    }
    if (segment === "..") {
      segments.pop();
      return;
    }
    segments.push(segment);
  });
  return segments.join("/");
}

function emu_to_pixel(value: number): number {
  return value / EMU_PER_PIXEL;
}

function children_by_local_name(element: Element | null, local_name: string): Element[] {
  if (!element) {
    return [];
  }
  return Array.from(element.children).filter((child) => child.localName === local_name);
}

function first_child_by_local_name(element: Element | null, local_name: string): Element | null {
  return children_by_local_name(element, local_name)[0] || null;
}

function descendants_by_local_name(root: Document | Element, local_name: string): Element[] {
  return Array.from(root.getElementsByTagName("*")).filter((element) => element.localName === local_name);
}

function first_descendant_by_local_name(root: Document | Element | null, local_name: string): Element | null {
  if (!root) {
    return null;
  }
  return descendants_by_local_name(root, local_name)[0] || null;
}

function revoke_object_urls(urls: string[]) {
  urls.forEach((url) => URL.revokeObjectURL(url));
}
