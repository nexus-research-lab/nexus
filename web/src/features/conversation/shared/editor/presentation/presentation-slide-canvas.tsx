import { cn } from "@/shared/ui/class-name";
import {
  ROUND_RECT_MAX_RADIUS,
  ROUND_RECT_RADIUS_RATIO,
  type PresentationParagraph,
  type PresentationShapeElement,
  type PresentationSlide,
  type PresentationTextRun,
} from "./presentation-preview-model";

export function PresentationSlideCanvas({
  className: className,
  slide,
  thumbnail = false,
}: {
  className?: string;
  slide: PresentationSlide;
  thumbnail?: boolean;
}) {
  return (
    <svg
      aria-label={slide.title}
      className={cn(
        "block w-full bg-(--surface-paper-background) shadow-(--surface-paper-shadow)",
        thumbnail ? "rounded-[2px] shadow-sm" : "rounded-[2px]",
        className,
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
  const textPadding = thumbnail
    ? Math.max(Math.min(shape.width, shape.height) * 0.03, 4)
    : Math.max(Math.min(shape.width, shape.height) * 0.045, 6);
  const justifyContent = shape.textAnchor === "center"
    ? "center"
    : shape.textAnchor === "bottom"
      ? "flex-end"
      : "flex-start";

  return (
    <g>
      {renderShapeGeometry(shape, fill, stroke)}
      {shape.paragraphs.length > 0 ? (
        <foreignObject height={shape.height} width={shape.width} x={shape.x} y={shape.y}>
          <div
            style={{
              boxSizing: "border-box",
              display: "flex",
              flexDirection: "column",
              height: "100%",
              justifyContent,
              overflow: "hidden",
              padding: textPadding,
              width: "100%",
            }}
          >
            {shape.paragraphs.map((paragraph) => (
              <p
                key={getParagraphKey(shape.id, paragraph)}
                style={{
                  columnGap: paragraph.bullet ? paragraph.fontSize * 0.45 : undefined,
                  display: paragraph.bullet ? "grid" : "block",
                  fontSize: paragraph.fontSize,
                  gridTemplateColumns: paragraph.bullet ? `${paragraph.bulletIndent}px minmax(0, 1fr)` : undefined,
                  lineHeight: paragraph.lineHeight,
                  margin: paragraph === shape.paragraphs[0] ? 0 : `${paragraph.fontSize * 0.42}px 0 0`,
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
                      fontSize: paragraph.fontSize,
                      fontWeight: 700,
                      lineHeight: paragraph.lineHeight,
                    }}
                  >
                    {paragraph.bullet}
                  </span>
                ) : null}
                <span style={{ minWidth: 0, overflowWrap: paragraph.align === "center" ? "normal" : "break-word" }}>
                  {paragraph.runs.map((run) => (
                    <span
                      key={getTextRunKey(shape.id, paragraph, run)}
                      style={{
                        color: run.color || "#111827",
                        fontFamily: run.fontFace || "Arial, sans-serif",
                        fontSize: run.fontSize,
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

function getParagraphKey(shapeId: string, paragraph: PresentationParagraph): string {
  return [
    shapeId,
    "paragraph",
    paragraph.text,
    paragraph.bullet ?? "",
    paragraph.align ?? "",
    paragraph.fontSize,
    paragraph.lineHeight,
  ].join(":");
}

function getTextRunKey(
  shapeId: string,
  paragraph: PresentationParagraph,
  run: PresentationTextRun,
): string {
  return [
    getParagraphKey(shapeId, paragraph),
    "run",
    run.text,
    run.fontFace ?? "",
    run.fontSize,
    run.color ?? "",
    run.bold ? "bold" : "normal",
    run.italic ? "italic" : "roman",
  ].join(":");
}

function renderShapeGeometry(shape: PresentationShapeElement, fill: string, stroke: string) {
  if (shape.geometry === "unsupported") {
    return null;
  }

  const commonProps = {
    fill,
    stroke,
    strokeWidth: shape.stroke === undefined ? 0 : shape.strokeWidth,
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
          {...commonProps}
        />
      );
    case "ellipse":
      return (
        <ellipse
          cx={shape.x + shape.width / 2}
          cy={shape.y + shape.height / 2}
          rx={Math.abs(shape.width / 2)}
          ry={Math.abs(shape.height / 2)}
          {...commonProps}
        />
      );
    case "line":
      return (
        <line
          stroke={stroke === "none" ? "#64748b" : stroke}
          strokeWidth={Math.max(shape.strokeWidth, 1)}
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
          {...commonProps}
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
          {...commonProps}
        />
      );
    case "rect":
      return (
        <rect
          height={shape.height}
          width={shape.width}
          x={shape.x}
          y={shape.y}
          {...commonProps}
        />
      );
  }
}
