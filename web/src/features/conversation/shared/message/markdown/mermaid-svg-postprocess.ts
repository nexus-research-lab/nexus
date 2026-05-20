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

function clamp_rounded_rect_radius(width: number, height: number, radius: number): number {
  return Math.max(0, Math.min(radius, width / 2, height / 2));
}

function create_rounded_rect_path_d(
  x: number,
  y: number,
  width: number,
  height: number,
  radius: number,
): string {
  const rounded_radius = clamp_rounded_rect_radius(width, height, radius);

  return [
    `M ${x + rounded_radius} ${y}`,
    `H ${x + width - rounded_radius}`,
    `A ${rounded_radius} ${rounded_radius} 0 0 1 ${x + width} ${y + rounded_radius}`,
    `V ${y + height - rounded_radius}`,
    `A ${rounded_radius} ${rounded_radius} 0 0 1 ${x + width - rounded_radius} ${y + height}`,
    `H ${x + rounded_radius}`,
    `A ${rounded_radius} ${rounded_radius} 0 0 1 ${x} ${y + height - rounded_radius}`,
    `V ${y + rounded_radius}`,
    `A ${rounded_radius} ${rounded_radius} 0 0 1 ${x + rounded_radius} ${y}`,
    "Z",
  ].join(" ");
}

function create_rounded_polygon_path_d(points: string, radius: number): string | null {
  const vertices = points
    .trim()
    .split(/\s+/)
    .map((point) => {
      const [raw_x, raw_y] = point.split(",").map(Number);
      if (!Number.isFinite(raw_x) || !Number.isFinite(raw_y)) {
        return null;
      }
      return { x: raw_x, y: raw_y };
    });

  if (vertices.some((vertex) => vertex === null) || vertices.length < 3) {
    return null;
  }

  const safe_vertices = vertices as Array<{ x: number; y: number }>;
  const segments: string[] = [];

  for (let index = 0; index < safe_vertices.length; index += 1) {
    const previous = safe_vertices[(index - 1 + safe_vertices.length) % safe_vertices.length];
    const current = safe_vertices[index];
    const next = safe_vertices[(index + 1) % safe_vertices.length];

    if (!previous || !current || !next) {
      return null;
    }

    const previous_dx = previous.x - current.x;
    const previous_dy = previous.y - current.y;
    const next_dx = next.x - current.x;
    const next_dy = next.y - current.y;
    const previous_length = Math.hypot(previous_dx, previous_dy);
    const next_length = Math.hypot(next_dx, next_dy);

    if (previous_length === 0 || next_length === 0) {
      return null;
    }

    const safe_radius = Math.min(radius, previous_length / 2, next_length / 2);
    const start_x = current.x + (previous_dx / previous_length) * safe_radius;
    const start_y = current.y + (previous_dy / previous_length) * safe_radius;
    const end_x = current.x + (next_dx / next_length) * safe_radius;
    const end_y = current.y + (next_dy / next_length) * safe_radius;

    segments.push(index === 0 ? `M ${start_x} ${start_y}` : `L ${start_x} ${start_y}`);
    segments.push(`Q ${current.x} ${current.y} ${end_x} ${end_y}`);
  }

  segments.push("Z");
  return segments.join(" ");
}

function extract_rectangle_bounds_from_path(path_data: string): RectangleBounds | null {
  const numbers = path_data.match(/-?\d*\.?\d+/g)?.map(Number);
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

  const unique_x = new Set(points.map((point) => Math.round(point.x * 100) / 100));
  const unique_y = new Set(points.map((point) => Math.round(point.y * 100) / 100));
  if (unique_x.size > 2 || unique_y.size > 2) {
    return null;
  }

  const x_values = points.map((point) => point.x);
  const y_values = points.map((point) => point.y);
  const x = Math.min(...x_values);
  const y = Math.min(...y_values);

  return {
    height: Math.max(...y_values) - y,
    width: Math.max(...x_values) - x,
    x,
    y,
  };
}

function set_rect_rounding(rect: SVGRectElement, radius: number): void {
  rect.setAttribute("rx", String(radius));
  rect.setAttribute("ry", String(radius));
}

function append_mermaid_svg_style(root: SVGSVGElement): void {
  let style_el = root.querySelector<SVGStyleElement>("style");
  if (!style_el) {
    style_el = root.ownerDocument.createElementNS(SVG_NS, "style") as SVGStyleElement;
    root.insertBefore(style_el, root.firstChild);
  }

  style_el.textContent = `${style_el.textContent ?? ""}
.edgeLabel, .edgeLabel p { background-color: transparent !important; }
.edgeLabel rect { opacity: 1 !important; }
.labelBkg { background-color: transparent !important; box-shadow: none !important; }
.nodeLabel, .edgeLabel, .cluster-label, .messageText, .actor {
  font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "PingFang SC", "Microsoft YaHei", sans-serif !important;
}`;
}

function soften_edge_labels(root: SVGSVGElement, tokens: MermaidSvgVisualTokens): void {
  root.querySelectorAll<SVGRectElement>(".edgeLabel rect, rect.labelBox").forEach((rect) => {
    set_rect_rounding(rect, tokens.edge_label_radius);
    rect.setAttribute("fill", tokens.edge_label_background);
    rect.setAttribute("stroke", tokens.edge_label_border);
  });

  root.querySelectorAll<SVGTextElement>(".edgeLabel text, .edgeLabel tspan").forEach((text) => {
    text.setAttribute("fill", tokens.edge_label_text);
  });
}

function soften_note_nodes(root: SVGSVGElement, tokens: MermaidSvgVisualTokens): void {
  root.querySelectorAll<SVGRectElement>("rect.note, .note rect").forEach((rect) => {
    set_rect_rounding(rect, tokens.border_radius);
    rect.setAttribute("fill", tokens.note_background);
    rect.setAttribute("stroke", tokens.note_border);
  });

  root.querySelectorAll<SVGTextElement>(".noteText, .note text").forEach((text) => {
    text.setAttribute("fill", tokens.note_text);
  });
}

function round_rectangle_paths(root: SVGSVGElement, radius: number): void {
  root
    .querySelectorAll<SVGPathElement>(".basic.label-container path, g.basic.label-container path, .node.note path")
    .forEach((path) => {
      const path_data = path.getAttribute("d");
      if (!path_data) {
        return;
      }

      const bounds = extract_rectangle_bounds_from_path(path_data);
      if (!bounds || bounds.width <= 0 || bounds.height <= 0) {
        return;
      }

      path.setAttribute(
        "d",
        create_rounded_rect_path_d(bounds.x, bounds.y, bounds.width, bounds.height, radius),
      );
    });
}

function round_rect_nodes(root: SVGSVGElement, radius: number): void {
  const rounded_rect_selectors = [
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

  root.querySelectorAll<SVGRectElement>(rounded_rect_selectors.join(", ")).forEach((rect) => {
    set_rect_rounding(rect, radius);
  });
}

function round_polygon_nodes(root: SVGSVGElement, radius: number): void {
  root.querySelectorAll<SVGPolygonElement>(".node polygon").forEach((polygon) => {
    const points = polygon.getAttribute("points");
    if (!points) {
      return;
    }

    const path_data = create_rounded_polygon_path_d(points, radius);
    if (!path_data) {
      return;
    }

    const path = root.ownerDocument.createElementNS(SVG_NS, "path");
    path.setAttribute("d", path_data);
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

    const svg_root = root as unknown as SVGSVGElement;
    append_mermaid_svg_style(svg_root);
    soften_edge_labels(svg_root, MERMAID_SVG_TOKENS);
    soften_note_nodes(svg_root, MERMAID_SVG_TOKENS);
    round_rectangle_paths(svg_root, MERMAID_SVG_TOKENS.border_radius);
    round_rect_nodes(svg_root, MERMAID_SVG_TOKENS.border_radius);
    round_polygon_nodes(svg_root, MERMAID_SVG_TOKENS.border_radius);

    return new XMLSerializer().serializeToString(document);
  } catch {
    return svg;
  }
}
