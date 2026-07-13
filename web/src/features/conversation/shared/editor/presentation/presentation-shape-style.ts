import {
  SCHEME_COLORS,
  type PresentationElement,
  type PresentationGroupTransform,
  type PresentationPlaceholderStyle,
  type PresentationShapeGeometry,
  type PresentationTransform,
} from "./presentation-preview-model";
import {
  emuToPixel,
  firstChildByLocalName,
  firstDescendantByLocalName,
} from "./presentation-xml-utils";

type SolidFillColorReader = (solidFill: Element) => string | undefined;

const SHAPE_GEOMETRY_BY_PRESET: Record<string, PresentationShapeGeometry> = {
  diamond: "diamond",
  ellipse: "ellipse",
  line: "line",
  rect: "rect",
  roundRect: "roundRect",
  rtTriangle: "triangle",
  triangle: "triangle",
};

const PRESET_FILL_COLORS: Record<string, string> = {
  black: "#000000",
  white: "#ffffff",
};

const SOLID_FILL_COLOR_READERS: SolidFillColorReader[] = [
  readSrgbFillColor,
  readSystemFillColor,
  readPresetFillColor,
  readSchemeFillColor,
];

export function readGroupTransform(element: Element): PresentationGroupTransform | null {
  const groupProperties = firstChildByLocalName(element, "grpSpPr");
  const transform = firstChildByLocalName(groupProperties, "xfrm");
  const groupRect = readTransformRect(transform);
  const childRectElements = readRequiredChildren(transform, ["chOff", "chExt"]);
  if (!groupRect || !childRectElements) {
    return null;
  }

  const childRect = createTransformRect(
    childRectElements[0],
    childRectElements[1],
  );
  if (!hasPositiveArea(groupRect) || !hasPositiveArea(childRect)) {
    return null;
  }

  return {
    ...groupRect,
    childHeight: childRect.height,
    childWidth: childRect.width,
    childX: childRect.x,
    childY: childRect.y,
  };
}

export function applyGroupTransformToElement(
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
      bulletIndent: paragraph.bulletIndent * scale,
      fontSize: paragraph.fontSize * scale,
      runs: paragraph.runs.map((run) => ({
        ...run,
        fontSize: run.fontSize * scale,
      })),
    })),
    strokeWidth: element.strokeWidth * scale,
  };
}

export function mapGroupPlaceholderStyles(
  placeholderStyles: Map<string, PresentationPlaceholderStyle>,
  groupTransform: PresentationGroupTransform,
): Map<string, PresentationPlaceholderStyle> {
  const scale = groupScale(groupTransform);
  return new Map(Array.from(placeholderStyles.entries()).map(([key, style]) => {
    return [key, {
      ...style,
      strokeWidth: style.strokeWidth * scale,
      transform: applyGroupTransformToRect(style.transform, groupTransform),
    }];
  }));
}

function applyGroupTransformToRect(
  transform: PresentationTransform,
  groupTransform: PresentationGroupTransform,
): PresentationTransform {
  const scaleX = groupTransform.width / groupTransform.childWidth;
  const scaleY = groupTransform.height / groupTransform.childHeight;
  return {
    height: transform.height * scaleY,
    width: transform.width * scaleX,
    x: groupTransform.x + ((transform.x - groupTransform.childX) * scaleX),
    y: groupTransform.y + ((transform.y - groupTransform.childY) * scaleY),
  };
}

function groupScale(groupTransform: PresentationGroupTransform): number {
  return Math.min(
    groupTransform.width / groupTransform.childWidth,
    groupTransform.height / groupTransform.childHeight,
  );
}

export function readTransform(shapeProperties: Element | null): PresentationTransform | null {
  const transform = firstChildByLocalName(shapeProperties, "xfrm")
    ?? firstDescendantByLocalName(shapeProperties, "xfrm");
  return readTransformRect(transform);
}

function readTransformRect(transform: Element | null): PresentationTransform | null {
  const rectElements = readRequiredChildren(transform, ["off", "ext"]);
  return rectElements
    ? createTransformRect(rectElements[0], rectElements[1])
    : null;
}

function readRequiredChildren(
  parent: Element | null,
  localNames: readonly string[],
): Element[] | null {
  const children: Element[] = [];
  for (const localName of localNames) {
    const child = firstChildByLocalName(parent, localName);
    if (!child) {
      return null;
    }
    children.push(child);
  }
  return children;
}

function createTransformRect(
  offset: Element,
  extent: Element,
): PresentationTransform {
  return {
    height: readEmuAttribute(extent, "cy"),
    width: readEmuAttribute(extent, "cx"),
    x: readEmuAttribute(offset, "x"),
    y: readEmuAttribute(offset, "y"),
  };
}

function readEmuAttribute(element: Element, name: string): number {
  return emuToPixel(Number(element.getAttribute(name) ?? 0));
}

function hasPositiveArea(transform: PresentationTransform): boolean {
  return transform.width > 0 && transform.height > 0;
}

export function readShapeGeometry(
  shapeProperties: Element | null,
  isConnector: boolean,
  fallbackGeometry?: PresentationShapeGeometry,
): PresentationShapeGeometry {
  if (isConnector) {
    return "line";
  }

  const presetGeometry = firstChildByLocalName(shapeProperties, "prstGeom");
  const preset = presetGeometry?.getAttribute("prst");
  const geometry = preset === null || preset === undefined
    ? undefined
    : SHAPE_GEOMETRY_BY_PRESET[preset];
  return geometry ?? fallbackGeometry ?? "unsupported";
}

export function readSlideBackground(slideDoc: Document): string | undefined {
  const background = firstDescendantByLocalName(slideDoc, "bgPr");
  return readFillColor(background);
}

export function readFillColor(element: Element | null): string | undefined {
  if (!element || firstChildByLocalName(element, "noFill")) {
    return undefined;
  }

  const solidFill = firstDescendantByLocalName(element, "solidFill");
  if (!solidFill) {
    return undefined;
  }

  for (const readColor of SOLID_FILL_COLOR_READERS) {
    const color = readColor(solidFill);
    if (color) {
      return color;
    }
  }
  return undefined;
}

function readSrgbFillColor(solidFill: Element): string | undefined {
  const colorElement = firstChildByLocalName(solidFill, "srgbClr");
  const value = colorElement?.getAttribute("val");
  return value ? applyColorLuminance(`#${value}`, colorElement) : undefined;
}

function readSystemFillColor(solidFill: Element): string | undefined {
  const colorElement = firstChildByLocalName(solidFill, "sysClr");
  const value = colorElement?.getAttribute("lastClr");
  return value ? `#${value}` : undefined;
}

function readPresetFillColor(solidFill: Element): string | undefined {
  const value = firstChildByLocalName(solidFill, "prstClr")?.getAttribute("val");
  return value ? PRESET_FILL_COLORS[value] : undefined;
}

function readSchemeFillColor(solidFill: Element): string | undefined {
  const colorElement = firstChildByLocalName(solidFill, "schemeClr");
  const value = colorElement?.getAttribute("val");
  return value
    ? applyColorLuminance(SCHEME_COLORS[value], colorElement)
    : undefined;
}

function applyColorLuminance(color: string | undefined, colorElement: Element | null): string | undefined {
  if (!color) {
    return undefined;
  }

  const lumMod = readColorAdjustment(colorElement, "lumMod", 100000);
  const lumOff = readColorAdjustment(colorElement, "lumOff", 0);
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

function readColorAdjustment(
  colorElement: Element | null,
  localName: string,
  fallback: number,
): number {
  const value = firstChildByLocalName(colorElement, localName)?.getAttribute("val");
  return Number(value ?? fallback);
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

export function readStrokeColor(shapeProperties: Element | null): string | undefined {
  const line = firstChildByLocalName(shapeProperties, "ln");
  if (!line || firstChildByLocalName(line, "noFill")) {
    return undefined;
  }
  return readFillColor(line) ?? "#64748b";
}

export function readStrokeWidth(
  shapeProperties: Element | null,
): number | undefined {
  const line = firstChildByLocalName(shapeProperties, "ln");
  if (!line) {
    return undefined;
  }
  const width = Number(line.getAttribute("w") ?? 0);
  return width > 0 ? Math.max(emuToPixel(width), 1) : 1;
}
