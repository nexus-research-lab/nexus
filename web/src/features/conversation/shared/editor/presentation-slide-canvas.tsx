import { cn } from "@/lib/utils";
import {
  ROUND_RECT_MAX_RADIUS,
  ROUND_RECT_RADIUS_RATIO,
  type PresentationParagraph,
  type PresentationShapeElement,
  type PresentationSlide,
  type PresentationTextRun,
} from "./presentation-preview-model";

export function PresentationSlideCanvas({
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
        "block w-full bg-(--surface-paper-background) shadow-(--surface-paper-shadow)",
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
            {shape.paragraphs.map((paragraph) => (
              <p
                key={get_paragraph_key(shape.id, paragraph)}
                style={{
                  columnGap: paragraph.bullet ? paragraph.font_size * 0.45 : undefined,
                  display: paragraph.bullet ? "grid" : "block",
                  fontSize: paragraph.font_size,
                  gridTemplateColumns: paragraph.bullet ? `${paragraph.bullet_indent}px minmax(0, 1fr)` : undefined,
                  lineHeight: paragraph.line_height,
                  margin: paragraph === shape.paragraphs[0] ? 0 : `${paragraph.font_size * 0.42}px 0 0`,
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
                  {paragraph.runs.map((run) => (
                    <span
                      key={get_text_run_key(shape.id, paragraph, run)}
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

function get_paragraph_key(shape_id: string, paragraph: PresentationParagraph): string {
  return [
    shape_id,
    "paragraph",
    paragraph.text,
    paragraph.bullet ?? "",
    paragraph.align ?? "",
    paragraph.font_size,
    paragraph.line_height,
  ].join(":");
}

function get_text_run_key(
  shape_id: string,
  paragraph: PresentationParagraph,
  run: PresentationTextRun,
): string {
  return [
    get_paragraph_key(shape_id, paragraph),
    "run",
    run.text,
    run.font_face ?? "",
    run.font_size,
    run.color ?? "",
    run.bold ? "bold" : "normal",
    run.italic ? "italic" : "roman",
  ].join(":");
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
