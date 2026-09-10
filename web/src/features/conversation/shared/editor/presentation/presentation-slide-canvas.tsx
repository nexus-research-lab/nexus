// INPUT: Parsed slide geometry and source text styles, with optional thumbnail chrome.
// OUTPUT: One SVG content layout at every display size, preserving repeated paragraphs and runs.
// POS: Presentation canvas; source formatting stays local and thumbnails only change outer chrome.
import type { CSSProperties } from "react";

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
  className,
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
        "block w-full rounded-[2px] bg-(--surface-paper-background) shadow-(--surface-paper-shadow)",
        thumbnail && "shadow-sm",
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

        return <PresentationShape key={element.id} shape={element} />;
      })}
    </svg>
  );
}

function PresentationShape({
  shape,
}: {
  shape: PresentationShapeElement;
}) {
  const stroke = shape.stroke || "none";
  const fill = shape.geometry === "line" ? "none" : shape.fill || "transparent";
  return (
    <g>
      {renderShapeGeometry(shape, fill, stroke)}
      <PresentationShapeText shape={shape} />
    </g>
  );
}

function PresentationShapeText({
  shape,
}: {
  shape: PresentationShapeElement;
}) {
  if (shape.paragraphs.length === 0) {
    return null;
  }
  const textPadding = Math.max(Math.min(shape.width, shape.height) * 0.045, 6);
  const justifyContent = shape.textAnchor === "center"
    ? "center"
    : shape.textAnchor === "bottom"
      ? "flex-end"
      : "flex-start";
  return (
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
        {shape.paragraphs.map((paragraph, index) => (
          <PresentationParagraphView
            // Parsed sequences are immutable and may repeat identical content; position is their identity.
            key={index}
            first={index === 0}
            paragraph={paragraph}
          />
        ))}
      </div>
    </foreignObject>
  );
}

function PresentationParagraphView({
  first,
  paragraph,
}: {
  first: boolean;
  paragraph: PresentationParagraph;
}) {
  return (
    <p style={buildPresentationParagraphStyle(paragraph, first)}>
      {paragraph.bullet ? (
        <span style={buildPresentationBulletStyle(paragraph)}>
          {paragraph.bullet}
        </span>
      ) : null}
      <span style={buildPresentationParagraphContentStyle(paragraph)}>
        {paragraph.runs.map((run, index) => (
          <PresentationTextRunView
            key={index}
            run={run}
          />
        ))}
      </span>
    </p>
  );
}

function buildPresentationParagraphStyle(
  paragraph: PresentationParagraph,
  first: boolean,
): CSSProperties {
  const centered = paragraph.align === "center";
  const hasBullet = Boolean(paragraph.bullet);
  return {
    columnGap: hasBullet ? paragraph.fontSize * 0.45 : undefined,
    display: hasBullet ? "grid" : "block",
    fontSize: paragraph.fontSize,
    gridTemplateColumns: hasBullet
      ? `${paragraph.bulletIndent}px minmax(0, 1fr)`
      : undefined,
    lineHeight: paragraph.lineHeight,
    margin: first ? 0 : `${paragraph.fontSize * 0.42}px 0 0`,
    textAlign: paragraph.align || "left",
    whiteSpace: "normal",
    wordBreak: centered ? "keep-all" : "normal",
  };
}

function buildPresentationBulletStyle(
  paragraph: PresentationParagraph,
): CSSProperties {
  return {
    color: paragraph.runs[0]?.color || "#111827",
    fontFamily: "Arial, sans-serif",
    fontSize: paragraph.fontSize,
    fontWeight: 700,
    lineHeight: paragraph.lineHeight,
  };
}

function buildPresentationParagraphContentStyle(
  paragraph: PresentationParagraph,
): CSSProperties {
  return {
    minWidth: 0,
    overflowWrap: paragraph.align === "center" ? "normal" : "break-word",
  };
}

function PresentationTextRunView({ run }: { run: PresentationTextRun }) {
  return (
    <span
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
  );
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
