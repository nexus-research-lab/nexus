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

export function readGroupTransform(element: Element): PresentationGroupTransform | null {
  const groupProperties = firstChildByLocalName(element, "grpSpPr");
  const transform = firstChildByLocalName(groupProperties, "xfrm");
  const offset = firstChildByLocalName(transform, "off");
  const extent = firstChildByLocalName(transform, "ext");
  const childOffset = firstChildByLocalName(transform, "chOff");
  const childExtent = firstChildByLocalName(transform, "chExt");
  if (!offset || !extent || !childOffset || !childExtent) {
    return null;
  }

  const childWidth = emuToPixel(Number(childExtent.getAttribute("cx") || 0));
  const childHeight = emuToPixel(Number(childExtent.getAttribute("cy") || 0));
  const width = emuToPixel(Number(extent.getAttribute("cx") || 0));
  const height = emuToPixel(Number(extent.getAttribute("cy") || 0));
  if (childWidth <= 0 || childHeight <= 0 || width <= 0 || height <= 0) {
    return null;
  }

  return {
    childHeight,
    childWidth,
    childX: emuToPixel(Number(childOffset.getAttribute("x") || 0)),
    childY: emuToPixel(Number(childOffset.getAttribute("y") || 0)),
    height,
    width,
    x: emuToPixel(Number(offset.getAttribute("x") || 0)),
    y: emuToPixel(Number(offset.getAttribute("y") || 0)),
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
  return new Map(Array.from(placeholderStyles.entries()).map(([key, style]) => {
    const scale = groupScale(groupTransform);
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
  const transform = firstChildByLocalName(shapeProperties, "xfrm") || firstDescendantByLocalName(shapeProperties, "xfrm");
  const offset = firstChildByLocalName(transform, "off");
  const extent = firstChildByLocalName(transform, "ext");
  if (!offset || !extent) {
    return null;
  }

  return {
    height: emuToPixel(Number(extent.getAttribute("cy") || 0)),
    width: emuToPixel(Number(extent.getAttribute("cx") || 0)),
    x: emuToPixel(Number(offset.getAttribute("x") || 0)),
    y: emuToPixel(Number(offset.getAttribute("y") || 0)),
  };
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

  const srgbColor = firstChildByLocalName(solidFill, "srgbClr");
  const srgbValue = srgbColor?.getAttribute("val");
  if (srgbValue) {
    return applyColorLuminance(`#${srgbValue}`, srgbColor);
  }

  const systemColor = firstChildByLocalName(solidFill, "sysClr");
  const systemValue = systemColor?.getAttribute("lastClr");
  if (systemValue) {
    return `#${systemValue}`;
  }

  const presetColor = firstChildByLocalName(solidFill, "prstClr");
  const presetValue = presetColor?.getAttribute("val");
  if (presetValue === "white") {
    return "#ffffff";
  }
  if (presetValue === "black") {
    return "#000000";
  }

  const schemeColor = firstChildByLocalName(solidFill, "schemeClr");
  const schemeValue = schemeColor?.getAttribute("val");
  return schemeValue ? applyColorLuminance(SCHEME_COLORS[schemeValue], schemeColor) : undefined;
}

function applyColorLuminance(color: string | undefined, colorElement: Element | null): string | undefined {
  if (!color) {
    return undefined;
  }

  const lumMod = Number(firstChildByLocalName(colorElement, "lumMod")?.getAttribute("val") || 100000);
  const lumOff = Number(firstChildByLocalName(colorElement, "lumOff")?.getAttribute("val") || 0);
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

export function readStrokeColor(shapeProperties: Element | null): string | undefined {
  const line = firstChildByLocalName(shapeProperties, "ln");
  if (!line || firstChildByLocalName(line, "noFill")) {
    return undefined;
  }
  return readFillColor(line) || "#64748b";
}

export function readStrokeWidth(shapeProperties: Element | null): number {
  const line = firstChildByLocalName(shapeProperties, "ln");
  const width = Number(line?.getAttribute("w") || 0);
  return width > 0 ? Math.max(emuToPixel(width), 1) : 1;
}
