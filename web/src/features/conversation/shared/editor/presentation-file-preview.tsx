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
  | "triangle";

interface PresentationParagraph {
  align?: "center" | "left" | "right";
  bold?: boolean;
  color?: string;
  font_face?: string;
  font_size: number;
  italic?: boolean;
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
                        "w-full rounded-lg border p-1.5 text-left transition-colors",
                        index === active_slide_index
                          ? "border-primary/45 bg-primary/8"
                          : "border-(--divider-subtle-color) bg-(--surface-panel-subtle-background) hover:border-primary/30",
                      )}
                      key={slide.id}
                      onClick={() => set_active_slide_index(index)}
                      type="button"
                    >
                      <PresentationSlideCanvas class_name="rounded-md shadow-none" slide={slide} thumbnail />
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
                        className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-(--divider-subtle-color) bg-(--surface-panel-background) text-(--text-default) transition-colors hover:border-primary/30 hover:text-primary disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
                        disabled={active_slide_index <= 0}
                        onClick={() => set_active_slide_index((index) => Math.max(index - 1, 0))}
                        type="button"
                      >
                        <ChevronLeft className="h-4 w-4" />
                      </button>
                      <button
                        aria-label="下一页幻灯片"
                        className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-(--divider-subtle-color) bg-(--surface-panel-background) text-(--text-default) transition-colors hover:border-primary/30 hover:text-primary disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
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
        thumbnail ? "shadow-sm" : "rounded-lg",
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

  return (
    <g>
      {render_shape_geometry(shape, fill, stroke)}
      {shape.paragraphs.length > 0 && !thumbnail ? (
        <foreignObject height={shape.height} width={shape.width} x={shape.x} y={shape.y}>
          <div
            style={{
              boxSizing: "border-box",
              display: "flex",
              flexDirection: "column",
              height: "100%",
              justifyContent: "center",
              overflow: "hidden",
              padding: Math.max(Math.min(shape.width, shape.height) * 0.045, 6),
              width: "100%",
            }}
          >
            {shape.paragraphs.map((paragraph, index) => (
              <p
                key={`${shape.id}-paragraph-${index}`}
                style={{
                  color: paragraph.color || "#111827",
                  fontFamily: paragraph.font_face || "Arial, sans-serif",
                  fontSize: paragraph.font_size,
                  fontStyle: paragraph.italic ? "italic" : "normal",
                  fontWeight: paragraph.bold ? 700 : 400,
                  lineHeight: 1.18,
                  margin: 0,
                  textAlign: paragraph.align || "left",
                  whiteSpace: "pre-wrap",
                }}
              >
                {paragraph.text}
              </p>
            ))}
          </div>
        </foreignObject>
      ) : null}
    </g>
  );
}

function render_shape_geometry(shape: PresentationShapeElement, fill: string, stroke: string) {
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
    case "roundRect":
      return (
        <rect
          height={shape.height}
          rx={Math.min(shape.width, shape.height) * 0.08}
          width={shape.width}
          x={shape.x}
          y={shape.y}
          {...common_props}
        />
      );
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
    default:
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
  const rels = await read_relationships(zip, slide_path);
  const background = read_slide_background(slide_doc);
  const shape_tree = first_descendant_by_local_name(slide_doc, "spTree");
  const elements = shape_tree ? await parse_shape_tree(zip, slide_path, rels, shape_tree, object_urls) : [];
  const first_text = elements
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

async function parse_shape_tree(
  zip: JSZip,
  slide_path: string,
  rels: Record<string, PresentationRelationship>,
  shape_tree: Element,
  object_urls: string[],
): Promise<PresentationElement[]> {
  const elements: PresentationElement[] = [];
  const children = Array.from(shape_tree.children);

  for (const child of children) {
    switch (child.localName) {
      case "cxnSp":
      case "sp": {
        const shape = parse_shape(child, `shape-${elements.length}`);
        if (shape) {
          elements.push(shape);
        }
        break;
      }
      case "grpSp": {
        const group_elements = await parse_shape_tree(zip, slide_path, rels, child, object_urls);
        elements.push(...group_elements);
        break;
      }
      case "pic": {
        const image = await parse_picture(zip, slide_path, rels, child, `image-${elements.length}`, object_urls);
        if (image) {
          elements.push(image);
        }
        break;
      }
      default:
        break;
    }
  }

  return elements;
}

function parse_shape(element: Element, id: string): PresentationShapeElement | null {
  const shape_properties = first_child_by_local_name(element, "spPr");
  const transform = read_transform(shape_properties);
  if (!transform) {
    return null;
  }

  const paragraphs = parse_text_body(first_child_by_local_name(element, "txBody"), transform.width);
  const fill = read_fill_color(shape_properties);
  const stroke = read_stroke_color(shape_properties);
  const stroke_width = read_stroke_width(shape_properties);
  const geometry = read_shape_geometry(shape_properties, element.localName === "cxnSp");

  if (!fill && !stroke && paragraphs.length === 0 && geometry !== "line") {
    return null;
  }

  return {
    ...transform,
    fill,
    geometry,
    id,
    paragraphs,
    stroke,
    stroke_width,
    type: "shape",
  };
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

  return children_by_local_name(text_body, "p")
    .map((paragraph) => {
      const paragraph_properties = first_child_by_local_name(paragraph, "pPr");
      const align = read_paragraph_align(paragraph_properties);
      const runs = children_by_local_name(paragraph, "r");
      const texts = runs
        .map((run) => first_descendant_by_local_name(run, "t")?.textContent || "")
        .join("");
      const fallback_text = first_descendant_by_local_name(paragraph, "t")?.textContent || "";
      const text = texts || fallback_text;
      const run_properties = runs
        .map((run) => first_child_by_local_name(run, "rPr"))
        .find((props): props is Element => !!props) || null;
      const font_size = read_font_size(run_properties, shape_width);

      return {
        align,
        bold: run_properties?.getAttribute("b") === "1",
        color: read_fill_color(run_properties) || "#111827",
        font_face: first_descendant_by_local_name(run_properties, "latin")?.getAttribute("typeface") || undefined,
        font_size,
        italic: run_properties?.getAttribute("i") === "1",
        text,
      };
    })
    .filter((paragraph) => paragraph.text.trim().length > 0);
}

function read_transform(shape_properties: Element | null): Omit<PresentationShapeElement, "fill" | "geometry" | "id" | "paragraphs" | "stroke" | "stroke_width" | "type"> | null {
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

function read_shape_geometry(shape_properties: Element | null, is_connector: boolean): PresentationShapeGeometry {
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
    case "roundRect":
      return "roundRect";
    case "triangle":
    case "rtTriangle":
      return "triangle";
    default:
      return "rect";
  }
}

function read_slide_background(slide_doc: Document): string {
  const background = first_descendant_by_local_name(slide_doc, "bgPr");
  return read_fill_color(background) || "#ffffff";
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
    return `#${srgb_value}`;
  }

  const scheme_color = first_child_by_local_name(solid_fill, "schemeClr");
  const scheme_value = scheme_color?.getAttribute("val");
  return scheme_value ? SCHEME_COLORS[scheme_value] : undefined;
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

function read_font_size(run_properties: Element | null, shape_width: number): number {
  const size = Number(run_properties?.getAttribute("sz") || 0);
  if (size > 0) {
    return Math.max((size / 100) * (96 / 72), 8);
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
