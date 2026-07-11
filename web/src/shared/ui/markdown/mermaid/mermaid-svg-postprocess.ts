"use client";

interface MermaidSvgVisualTokens {
  border_radius: number;
  edge_label_background: string;
  edge_label_border: string;
  edge_label_radius: number;
  edge_label_text: string;
  note_background: string;
  note_border: string;
  note_text: string;
}

interface RectangleBounds {
  height: number;
  width: number;
  x: number;
  y: number;
}

const SVG_NS = "http://www.w3.org/2000/svg";

const MERMAID_SVG_TOKENS: MermaidSvgVisualTokens = {
  border_radius: 8,
  edge_label_background: "#ffffff",
  edge_label_border: "#d8dee9",
  edge_label_radius: 7,
  edge_label_text: "#334155",
  note_background: "#fff7ed",
  note_border: "#fed7aa",
  note_text: "#7c2d12",
};

function clampRoundedRectRadius(width: number, height: number, radius: number): number {
  return Math.max(0, Math.min(radius, width / 2, height / 2));
}

function createRoundedRectPathD(
  x: number,
  y: number,
  width: number,
  height: number,
  radius: number,
): string {
  const roundedRadius = clampRoundedRectRadius(width, height, radius);

  return [
    `M ${x + roundedRadius} ${y}`,
    `H ${x + width - roundedRadius}`,
    `A ${roundedRadius} ${roundedRadius} 0 0 1 ${x + width} ${y + roundedRadius}`,
    `V ${y + height - roundedRadius}`,
    `A ${roundedRadius} ${roundedRadius} 0 0 1 ${x + width - roundedRadius} ${y + height}`,
    `H ${x + roundedRadius}`,
    `A ${roundedRadius} ${roundedRadius} 0 0 1 ${x} ${y + height - roundedRadius}`,
    `V ${y + roundedRadius}`,
    `A ${roundedRadius} ${roundedRadius} 0 0 1 ${x + roundedRadius} ${y}`,
    "Z",
  ].join(" ");
}

function createRoundedPolygonPathD(points: string, radius: number): string | null {
  const vertices = points
    .trim()
    .split(/\s+/)
    .map((point) => {
      const [rawX, rawY] = point.split(",").map(Number);
      if (!Number.isFinite(rawX) || !Number.isFinite(rawY)) {
        return null;
      }
      return { x: rawX, y: rawY };
    });

  if (vertices.some((vertex) => vertex === null) || vertices.length < 3) {
    return null;
  }

  const safeVertices = vertices as Array<{ x: number; y: number }>;
  const segments: string[] = [];

  for (let index = 0; index < safeVertices.length; index += 1) {
    const previous = safeVertices[(index - 1 + safeVertices.length) % safeVertices.length];
    const current = safeVertices[index];
    const next = safeVertices[(index + 1) % safeVertices.length];

    if (!previous || !current || !next) {
      return null;
    }

    const previousDx = previous.x - current.x;
    const previousDy = previous.y - current.y;
    const nextDx = next.x - current.x;
    const nextDy = next.y - current.y;
    const previousLength = Math.hypot(previousDx, previousDy);
    const nextLength = Math.hypot(nextDx, nextDy);

    if (previousLength === 0 || nextLength === 0) {
      return null;
    }

    const safeRadius = Math.min(radius, previousLength / 2, nextLength / 2);
    const startX = current.x + (previousDx / previousLength) * safeRadius;
    const startY = current.y + (previousDy / previousLength) * safeRadius;
    const endX = current.x + (nextDx / nextLength) * safeRadius;
    const endY = current.y + (nextDy / nextLength) * safeRadius;

    segments.push(index === 0 ? `M ${startX} ${startY}` : `L ${startX} ${startY}`);
    segments.push(`Q ${current.x} ${current.y} ${endX} ${endY}`);
  }

  segments.push("Z");
  return segments.join(" ");
}

function extractRectangleBoundsFromPath(pathData: string): RectangleBounds | null {
  const numbers = pathData.match(/-?\d*\.?\d+/g)?.map(Number);
  if (!numbers || numbers.length < 8 || numbers.length % 2 !== 0) {
    return null;
  }

  const points: Array<{ x: number; y: number }> = [];
  for (let index = 0; index < numbers.length; index += 2) {
    const x = numbers[index];
    const y = numbers[index + 1];
    if (!Number.isFinite(x) || !Number.isFinite(y)) {
      return null;
    }
    points.push({ x, y });
  }

  const uniqueX = new Set(points.map((point) => Math.round(point.x * 100) / 100));
  const uniqueY = new Set(points.map((point) => Math.round(point.y * 100) / 100));
  if (uniqueX.size > 2 || uniqueY.size > 2) {
    return null;
  }

  const xValues = points.map((point) => point.x);
  const yValues = points.map((point) => point.y);
  const x = Math.min(...xValues);
  const y = Math.min(...yValues);

  return {
    height: Math.max(...yValues) - y,
    width: Math.max(...xValues) - x,
    x,
    y,
  };
}

function setRectRounding(rect: SVGRectElement, radius: number): void {
  rect.setAttribute("rx", String(radius));
  rect.setAttribute("ry", String(radius));
}

function appendMermaidSvgStyle(root: SVGSVGElement): void {
  let styleEl = root.querySelector<SVGStyleElement>("style");
  if (!styleEl) {
    styleEl = root.ownerDocument.createElementNS(SVG_NS, "style") as SVGStyleElement;
    root.insertBefore(styleEl, root.firstChild);
  }

  styleEl.textContent = `${styleEl.textContent ?? ""}
.edgeLabel, .edgeLabel p { background-color: transparent !important; }
.edgeLabel rect { opacity: 1 !important; }
.labelBkg { background-color: transparent !important; box-shadow: none !important; }
.nodeLabel, .edgeLabel, .cluster-label, .messageText, .actor {
  font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "PingFang SC", "Microsoft YaHei", sans-serif !important;
}`;
}

function softenEdgeLabels(root: SVGSVGElement, tokens: MermaidSvgVisualTokens): void {
  root.querySelectorAll<SVGRectElement>(".edgeLabel rect, rect.labelBox").forEach((rect) => {
    setRectRounding(rect, tokens.edge_label_radius);
    rect.setAttribute("fill", tokens.edge_label_background);
    rect.setAttribute("stroke", tokens.edge_label_border);
  });

  root.querySelectorAll<SVGTextElement>(".edgeLabel text, .edgeLabel tspan").forEach((text) => {
    text.setAttribute("fill", tokens.edge_label_text);
  });
}

function softenNoteNodes(root: SVGSVGElement, tokens: MermaidSvgVisualTokens): void {
  root.querySelectorAll<SVGRectElement>("rect.note, .note rect").forEach((rect) => {
    setRectRounding(rect, tokens.border_radius);
    rect.setAttribute("fill", tokens.note_background);
    rect.setAttribute("stroke", tokens.note_border);
  });

  root.querySelectorAll<SVGTextElement>(".noteText, .note text").forEach((text) => {
    text.setAttribute("fill", tokens.note_text);
  });
}

function roundRectanglePaths(root: SVGSVGElement, radius: number): void {
  root
    .querySelectorAll<SVGPathElement>(".basic.label-container path, g.basic.label-container path, .node.note path")
    .forEach((path) => {
      const pathData = path.getAttribute("d");
      if (!pathData) {
        return;
      }

      const bounds = extractRectangleBoundsFromPath(pathData);
      if (!bounds || bounds.width <= 0 || bounds.height <= 0) {
        return;
      }

      path.setAttribute(
        "d",
        createRoundedRectPathD(bounds.x, bounds.y, bounds.width, bounds.height, radius),
      );
    });
}

function roundRectNodes(root: SVGSVGElement, radius: number): void {
  const roundedRectSelectors = [
    ".node > rect",
    ".node rect",
    ".classGroup > rect",
    ".classGroup rect",
    ".cluster > rect",
    ".cluster rect",
    ".actor",
    ".activation0",
    ".activation1",
    ".activation2",
    ".stateGroup rect",
    ".statediagram-state rect",
  ];

  root.querySelectorAll<SVGRectElement>(roundedRectSelectors.join(", ")).forEach((rect) => {
    setRectRounding(rect, radius);
  });
}

function roundPolygonNodes(root: SVGSVGElement, radius: number): void {
  root.querySelectorAll<SVGPolygonElement>(".node polygon").forEach((polygon) => {
    const points = polygon.getAttribute("points");
    if (!points) {
      return;
    }

    const pathData = createRoundedPolygonPathD(points, radius);
    if (!pathData) {
      return;
    }

    const path = root.ownerDocument.createElementNS(SVG_NS, "path");
    path.setAttribute("d", pathData);
    Array.from(polygon.attributes).forEach((attribute) => {
      if (attribute.name !== "points") {
        path.setAttribute(attribute.name, attribute.value);
      }
    });

    polygon.replaceWith(path);
  });
}

export function postProcessMermaidSvg(svg: string): string {
  if (!svg || typeof DOMParser === "undefined" || typeof XMLSerializer === "undefined") {
    return svg;
  }

  try {
    const document = new DOMParser().parseFromString(svg, "image/svg+xml");
    if (document.querySelector("parsererror")) {
      return svg;
    }

    const root = document.documentElement;
    if (!root || root.localName !== "svg") {
      return svg;
    }

    const svgRoot = root as unknown as SVGSVGElement;
    appendMermaidSvgStyle(svgRoot);
    softenEdgeLabels(svgRoot, MERMAID_SVG_TOKENS);
    softenNoteNodes(svgRoot, MERMAID_SVG_TOKENS);
    roundRectanglePaths(svgRoot, MERMAID_SVG_TOKENS.border_radius);
    roundRectNodes(svgRoot, MERMAID_SVG_TOKENS.border_radius);
    roundPolygonNodes(svgRoot, MERMAID_SVG_TOKENS.border_radius);

    return new XMLSerializer().serializeToString(document);
  } catch {
    return svg;
  }
}
