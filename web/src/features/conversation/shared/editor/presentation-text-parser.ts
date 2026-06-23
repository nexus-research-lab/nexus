import type {
  PresentationParagraph,
  PresentationTextRun,
} from "./presentation-preview-model";
import { read_fill_color } from "./presentation-shape-style";
import {
  children_by_local_name,
  emu_to_pixel,
  first_child_by_local_name,
  first_descendant_by_local_name,
} from "./presentation-xml-utils";

export function parse_text_body(text_body: Element | null, shape_width: number): PresentationParagraph[] {
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
