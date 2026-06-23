import {
  SCHEME_COLORS,
  type PresentationElement,
  type PresentationGroupTransform,
  type PresentationPlaceholderStyle,
  type PresentationShapeGeometry,
  type PresentationTransform,
} from "./presentation-preview-model";
import {
  emu_to_pixel,
  first_child_by_local_name,
  first_descendant_by_local_name,
} from "./presentation-xml-utils";

export function read_group_transform(element: Element): PresentationGroupTransform | null {
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

export function apply_group_transform_to_element(
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

export function map_group_placeholder_styles(
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

export function read_transform(shape_properties: Element | null): PresentationTransform | null {
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

export function read_shape_geometry(
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

export function read_slide_background(slide_doc: Document): string | undefined {
  const background = first_descendant_by_local_name(slide_doc, "bgPr");
  return read_fill_color(background);
}

export function read_fill_color(element: Element | null): string | undefined {
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

export function read_stroke_color(shape_properties: Element | null): string | undefined {
  const line = first_child_by_local_name(shape_properties, "ln");
  if (!line || first_child_by_local_name(line, "noFill")) {
    return undefined;
  }
  return read_fill_color(line) || "#64748b";
}

export function read_stroke_width(shape_properties: Element | null): number {
  const line = first_child_by_local_name(shape_properties, "ln");
  const width = Number(line?.getAttribute("w") || 0);
  return width > 0 ? Math.max(emu_to_pixel(width), 1) : 1;
}
