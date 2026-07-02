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
  const groupProperties = first_child_by_local_name(element, "grpSpPr");
  const transform = first_child_by_local_name(groupProperties, "xfrm");
  const offset = first_child_by_local_name(transform, "off");
  const extent = first_child_by_local_name(transform, "ext");
  const childOffset = first_child_by_local_name(transform, "chOff");
  const childExtent = first_child_by_local_name(transform, "chExt");
  if (!offset || !extent || !childOffset || !childExtent) {
    return null;
  }

  const childWidth = emu_to_pixel(Number(childExtent.getAttribute("cx") || 0));
  const childHeight = emu_to_pixel(Number(childExtent.getAttribute("cy") || 0));
  const width = emu_to_pixel(Number(extent.getAttribute("cx") || 0));
  const height = emu_to_pixel(Number(extent.getAttribute("cy") || 0));
  if (childWidth <= 0 || childHeight <= 0 || width <= 0 || height <= 0) {
    return null;
  }

  return {
    child_height: childHeight,
    child_width: childWidth,
    child_x: emu_to_pixel(Number(childOffset.getAttribute("x") || 0)),
    child_y: emu_to_pixel(Number(childOffset.getAttribute("y") || 0)),
    height,
    width,
    x: emu_to_pixel(Number(offset.getAttribute("x") || 0)),
    y: emu_to_pixel(Number(offset.getAttribute("y") || 0)),
  };
}

export function apply_group_transform_to_element(
  element: PresentationElement,
  groupTransform: PresentationGroupTransform,
): PresentationElement {
  const transform = applyGroupTransformToRect(element, groupTransform);
  if (element.type === "image") {
    return {
      ...element,
      ...transform,
    };
  }

  const scale = groupScale(groupTransform);
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
  placeholderStyles: Map<string, PresentationPlaceholderStyle>,
  groupTransform: PresentationGroupTransform,
): Map<string, PresentationPlaceholderStyle> {
  return new Map(Array.from(placeholderStyles.entries()).map(([key, style]) => {
    const scale = groupScale(groupTransform);
    return [key, {
      ...style,
      stroke_width: style.stroke_width * scale,
      transform: applyGroupTransformToRect(style.transform, groupTransform),
    }];
  }));
}

function applyGroupTransformToRect(
  transform: PresentationTransform,
  groupTransform: PresentationGroupTransform,
): PresentationTransform {
  const scaleX = groupTransform.width / groupTransform.child_width;
  const scaleY = groupTransform.height / groupTransform.child_height;
  return {
    height: transform.height * scaleY,
    width: transform.width * scaleX,
    x: groupTransform.x + ((transform.x - groupTransform.child_x) * scaleX),
    y: groupTransform.y + ((transform.y - groupTransform.child_y) * scaleY),
  };
}

function groupScale(groupTransform: PresentationGroupTransform): number {
  return Math.min(
    groupTransform.width / groupTransform.child_width,
    groupTransform.height / groupTransform.child_height,
  );
}

export function read_transform(shapeProperties: Element | null): PresentationTransform | null {
  const transform = first_child_by_local_name(shapeProperties, "xfrm") || first_descendant_by_local_name(shapeProperties, "xfrm");
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
  shapeProperties: Element | null,
  isConnector: boolean,
  fallbackGeometry?: PresentationShapeGeometry,
): PresentationShapeGeometry {
  if (isConnector) {
    return "line";
  }

  const presetGeometry = first_child_by_local_name(shapeProperties, "prstGeom");
  const preset = presetGeometry?.getAttribute("prst");
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
      return fallbackGeometry || "unsupported";
  }
}

export function read_slide_background(slideDoc: Document): string | undefined {
  const background = first_descendant_by_local_name(slideDoc, "bgPr");
  return read_fill_color(background);
}

export function read_fill_color(element: Element | null): string | undefined {
  if (!element || first_child_by_local_name(element, "noFill")) {
    return undefined;
  }

  const solidFill = first_descendant_by_local_name(element, "solidFill");
  if (!solidFill) {
    return undefined;
  }

  const srgbColor = first_child_by_local_name(solidFill, "srgbClr");
  const srgbValue = srgbColor?.getAttribute("val");
  if (srgbValue) {
    return applyColorLuminance(`#${srgbValue}`, srgbColor);
  }

  const systemColor = first_child_by_local_name(solidFill, "sysClr");
  const systemValue = systemColor?.getAttribute("lastClr");
  if (systemValue) {
    return `#${systemValue}`;
  }

  const presetColor = first_child_by_local_name(solidFill, "prstClr");
  const presetValue = presetColor?.getAttribute("val");
  if (presetValue === "white") {
    return "#ffffff";
  }
  if (presetValue === "black") {
    return "#000000";
  }

  const schemeColor = first_child_by_local_name(solidFill, "schemeClr");
  const schemeValue = schemeColor?.getAttribute("val");
  return schemeValue ? applyColorLuminance(SCHEME_COLORS[schemeValue], schemeColor) : undefined;
}

function applyColorLuminance(color: string | undefined, colorElement: Element | null): string | undefined {
  if (!color) {
    return undefined;
  }

  const lumMod = Number(first_child_by_local_name(colorElement, "lumMod")?.getAttribute("val") || 100000);
  const lumOff = Number(first_child_by_local_name(colorElement, "lumOff")?.getAttribute("val") || 0);
  if (lumMod === 100000 && lumOff === 0) {
    return color;
  }

  const rgb = parseHexColor(color);
  if (!rgb) {
    return color;
  }

  const channels = rgb.map((channel) => clampColorChannel(
    (channel * lumMod / 100000) + (255 * lumOff / 100000),
  ));
  return `#${channels.map((channel) => channel.toString(16).padStart(2, "0")).join("")}`;
}

function parseHexColor(color: string): [number, number, number] | null {
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

function clampColorChannel(value: number): number {
  return Math.max(0, Math.min(255, Math.round(value)));
}

export function read_stroke_color(shapeProperties: Element | null): string | undefined {
  const line = first_child_by_local_name(shapeProperties, "ln");
  if (!line || first_child_by_local_name(line, "noFill")) {
    return undefined;
  }
  return read_fill_color(line) || "#64748b";
}

export function read_stroke_width(shapeProperties: Element | null): number {
  const line = first_child_by_local_name(shapeProperties, "ln");
  const width = Number(line?.getAttribute("w") || 0);
  return width > 0 ? Math.max(emu_to_pixel(width), 1) : 1;
}
