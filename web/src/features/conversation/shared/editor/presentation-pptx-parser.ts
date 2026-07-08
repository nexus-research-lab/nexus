import type JSZip from "jszip";

import {
  DEFAULT_SLIDE_HEIGHT_EMU,
  DEFAULT_SLIDE_WIDTH_EMU,
  MIN_BACKGROUND_LIKE_SHAPE_SIZE,
  MIN_DECORATION_SHAPE_SIZE,
  SLIDE_LAYOUT_RELATIONSHIP_TYPE,
  SLIDE_MASTER_RELATIONSHIP_TYPE,
  type PresentationElement,
  type PresentationGroupTransform,
  type PresentationImageElement,
  type PresentationParagraph,
  type PresentationParseResult,
  type PresentationPart,
  type PresentationPlaceholderStyle,
  type PresentationRelationship,
  type PresentationShapeElement,
  type PresentationShapeGeometry,
  type PresentationShapeTreeContext,
  type PresentationShapeTreeResult,
  type PresentationSlide,
} from "./presentation-preview-model";
import {
  applyGroupTransformToElement,
  mapGroupPlaceholderStyles,
  readFillColor,
  readGroupTransform,
  readShapeGeometry,
  readSlideBackground,
  readStrokeColor,
  readStrokeWidth,
  readTransform,
} from "./presentation-shape-style";
import { parseTextBody } from "./presentation-text-parser";
import {
  descendantsByLocalName,
  emuToPixel,
  firstChildByLocalName,
  firstDescendantByLocalName,
  parseXml,
  readRelationships,
  readZipText,
  relationshipAttribute,
  resolveRelationshipTarget,
  revokeObjectUrls,
} from "./presentation-xml-utils";

export async function parsePptx(buffer: ArrayBuffer): Promise<PresentationParseResult> {
  const { default: JSZipConstructor } = await import("jszip");
  const zip = await JSZipConstructor.loadAsync(buffer);
  const objectUrls: string[] = [];

  try {
    const presentationXml = await readZipText(zip, "ppt/presentation.xml");
    const presentationDoc = parseXml(presentationXml);
    const presentationRels = await readRelationships(zip, "ppt/presentation.xml");
    const { height, width } = readSlideSize(presentationDoc);
    const slidePaths = readSlidePaths(presentationDoc, presentationRels);
    const resolvedSlidePaths = slidePaths.length > 0 ? slidePaths : fallbackSlidePaths(zip);

    if (resolvedSlidePaths.length === 0) {
      throw new Error("pptx 文件中没有可预览的幻灯片");
    }

    const slides: PresentationSlide[] = [];
    for (let index = 0; index < resolvedSlidePaths.length; index += 1) {
      const slide = await parseSlide(zip, resolvedSlidePaths[index], index, width, height, objectUrls);
      slides.push(slide);
    }

    return { objectUrls, slides };
  } catch (error) {
    revokeObjectUrls(objectUrls);
    throw error;
  }
}

function readSlideSize(presentationDoc: Document): { height: number; width: number } {
  const slideSize = firstDescendantByLocalName(presentationDoc, "sldSz");
  const widthEmu = Number(slideSize?.getAttribute("cx") || DEFAULT_SLIDE_WIDTH_EMU);
  const heightEmu = Number(slideSize?.getAttribute("cy") || DEFAULT_SLIDE_HEIGHT_EMU);
  return {
    height: Math.max(emuToPixel(heightEmu), 1),
    width: Math.max(emuToPixel(widthEmu), 1),
  };
}

function readSlidePaths(
  presentationDoc: Document,
  presentationRels: Record<string, PresentationRelationship>,
): string[] {
  return descendantsByLocalName(presentationDoc, "sldId")
    .map((slideId) => {
      const relId = relationshipAttribute(slideId, "id");
      const rel = relId ? presentationRels[relId] : undefined;
      return rel ? resolveRelationshipTarget("ppt/presentation.xml", rel.target) : null;
    })
    .filter((path): path is string => !!path);
}

function fallbackSlidePaths(zip: JSZip): string[] {
  return Object.keys(zip.files)
    .filter((path) => /^ppt\/slides\/slide\d+\.xml$/i.test(path))
    .sort((left, right) => {
      const leftNumber = Number(left.match(/slide(\d+)\.xml$/i)?.[1] || 0);
      const rightNumber = Number(right.match(/slide(\d+)\.xml$/i)?.[1] || 0);
      return leftNumber - rightNumber;
    });
}

async function parseSlide(
  zip: JSZip,
  slidePath: string,
  index: number,
  width: number,
  height: number,
  objectUrls: string[],
): Promise<PresentationSlide> {
  const slideXml = await readZipText(zip, slidePath);
  const slideDoc = parseXml(slideXml);
  const slideRels = await readRelationships(zip, slidePath);
  const layoutPath = resolveRelatedPartPath(slidePath, slideRels, SLIDE_LAYOUT_RELATIONSHIP_TYPE);
  const layoutRels = layoutPath ? await readRelationships(zip, layoutPath) : {};
  const masterPath = layoutPath
    ? resolveRelatedPartPath(layoutPath, layoutRels, SLIDE_MASTER_RELATIONSHIP_TYPE)
    : null;
  const masterPart = masterPath ? await parsePresentationPart(zip, masterPath, objectUrls) : null;
  const layoutPart = layoutPath
    ? await parsePresentationPart(zip, layoutPath, objectUrls, masterPart?.placeholderStyles)
    : null;
  const inheritedPlaceholders = mergePlaceholderStyles(
    masterPart?.placeholderStyles,
    layoutPart?.placeholderStyles,
  );
  const background = readSlideBackground(slideDoc)
    || layoutPart?.background
    || masterPart?.background
    || "#ffffff";
  const shapeTree = firstDescendantByLocalName(slideDoc, "spTree");
  const slideResult = shapeTree ? await parseShapeTree(
    zip,
    slidePath,
    slideRels,
    shapeTree,
    objectUrls,
    {
      elementIndex: 0,
      fallbackPlaceholders: inheritedPlaceholders,
      idPrefix: `slide-${index + 1}`,
      includePlaceholderShapes: true,
    },
  ) : { elements: [], placeholderStyles: new Map<string, PresentationPlaceholderStyle>() };
  const elements = [
    ...(masterPart?.elements ?? []),
    ...(layoutPart?.elements ?? []),
    ...slideResult.elements,
  ];
  const firstText = slideResult.elements
    .flatMap((element) => element.type === "shape" ? element.paragraphs : [])
    .map((paragraph) => paragraph.text.trim())
    .find(Boolean);

  return {
    background,
    elements,
    height,
    id: `slide-${index + 1}`,
    title: firstText || `幻灯片 ${index + 1}`,
    width,
  };
}

async function parsePresentationPart(
  zip: JSZip,
  partPath: string,
  objectUrls: string[],
  fallbackPlaceholders?: Map<string, PresentationPlaceholderStyle>,
): Promise<PresentationPart | null> {
  if (!zip.file(partPath)) {
    return null;
  }

  const partXml = await readZipText(zip, partPath);
  const partDoc = parseXml(partXml);
  const rels = await readRelationships(zip, partPath);
  const shapeTree = firstDescendantByLocalName(partDoc, "spTree");
  const result = shapeTree ? await parseShapeTree(zip, partPath, rels, shapeTree, objectUrls, {
    elementIndex: 0,
    fallbackPlaceholders: fallbackPlaceholders,
    idPrefix: partPath.replace(/[^a-z0-9]+/gi, "-"),
    includePlaceholderShapes: false,
  }) : { elements: [], placeholderStyles: new Map<string, PresentationPlaceholderStyle>() };

  return {
    background: readSlideBackground(partDoc),
    elements: result.elements,
    placeholderStyles: result.placeholderStyles,
    rels,
  };
}

function resolveRelatedPartPath(
  sourcePath: string,
  sourceRels: Record<string, PresentationRelationship>,
  relationshipType: string,
): string | null {
  const rel = Object.values(sourceRels).find((relationship) => relationship.type === relationshipType);
  if (!rel || rel.targetMode === "External") {
    return null;
  }
  return resolveRelationshipTarget(sourcePath, rel.target);
}

function mergePlaceholderStyles(
  base?: Map<string, PresentationPlaceholderStyle>,
  override?: Map<string, PresentationPlaceholderStyle>,
): Map<string, PresentationPlaceholderStyle> {
  return new Map([
    ...(base?.entries() ?? []),
    ...(override?.entries() ?? []),
  ]);
}

async function parseShapeTree(
  zip: JSZip,
  slidePath: string,
  rels: Record<string, PresentationRelationship>,
  shapeTree: Element,
  objectUrls: string[],
  context: PresentationShapeTreeContext,
  groupTransform?: PresentationGroupTransform | null,
): Promise<PresentationShapeTreeResult> {
  const elements: PresentationElement[] = [];
  const placeholderStyles = new Map<string, PresentationPlaceholderStyle>();
  const children = Array.from(shapeTree.children);

  for (const child of children) {
    switch (child.localName) {
      case "cxnSp":
      case "sp": {
        const parsedShape = parseShape(child, `${context.idPrefix}-shape-${context.elementIndex}`, context);
        context.elementIndex += 1;
        if (parsedShape.placeholderStyle) {
          placeholderStyles.set(parsedShape.placeholderStyle.key, parsedShape.placeholderStyle);
        }
        if (parsedShape.shape && (!parsedShape.isPlaceholder || context.includePlaceholderShapes)) {
          elements.push(parsedShape.shape);
        }
        break;
      }
      case "grpSp": {
        const groupResult = await parseShapeTree(
          zip,
          slidePath,
          rels,
          child,
          objectUrls,
          context,
          readGroupTransform(child),
        );
        groupResult.placeholderStyles.forEach((style, key) => {
          placeholderStyles.set(key, style);
        });
        elements.push(...groupResult.elements);
        break;
      }
      case "pic": {
        const image = await parsePicture(
          zip,
          slidePath,
          rels,
          child,
          `${context.idPrefix}-image-${context.elementIndex}`,
          objectUrls,
        );
        context.elementIndex += 1;
        if (image) {
          elements.push(image);
        }
        break;
      }
      default:
        break;
    }
  }

  if (!groupTransform) {
    return { elements, placeholderStyles };
  }

  return {
    elements: elements.map((element) => applyGroupTransformToElement(element, groupTransform)),
    placeholderStyles: mapGroupPlaceholderStyles(placeholderStyles, groupTransform),
  };
}

function parseShape(
  element: Element,
  id: string,
  context: PresentationShapeTreeContext,
): {
  isPlaceholder: boolean;
  placeholderStyle: PresentationPlaceholderStyle | null;
  shape: PresentationShapeElement | null;
} {
  const shapeProperties = firstChildByLocalName(element, "spPr");
  const placeholderKey = readPlaceholderKey(element);
  const fallbackPlaceholder = placeholderKey ? context.fallbackPlaceholders?.get(placeholderKey) : undefined;
  const transform = readTransform(shapeProperties) || fallbackPlaceholder?.transform || null;
  if (!transform) {
    return {
      isPlaceholder: !!placeholderKey,
      placeholderStyle: null,
      shape: null,
    };
  }

  const textBody = firstChildByLocalName(element, "txBody");
  const paragraphs = parseTextBody(textBody, transform.width);
  const textAnchor = readTextAnchor(firstChildByLocalName(textBody, "bodyPr"));
  const fill = readFillColor(shapeProperties) || fallbackPlaceholder?.fill;
  const stroke = readStrokeColor(shapeProperties) || fallbackPlaceholder?.stroke;
  const strokeWidth = readStrokeWidth(shapeProperties) || fallbackPlaceholder?.strokeWidth || 1;
  const geometry = readShapeGeometry(shapeProperties, element.localName === "cxnSp", fallbackPlaceholder?.geometry);
  const placeholderStyle = placeholderKey ? {
    fill,
    geometry,
    key: placeholderKey,
    stroke,
    strokeWidth,
    transform,
  } : null;

  if (shouldSkipShapePreview({ fill, geometry, height: transform.height, paragraphs, stroke, width: transform.width })) {
    return {
      isPlaceholder: !!placeholderKey,
      placeholderStyle,
      shape: null,
    };
  }

  return {
    isPlaceholder: !!placeholderKey,
    placeholderStyle,
    shape: {
      ...transform,
      fill,
      geometry,
      id,
      paragraphs,
      stroke,
      strokeWidth,
      textAnchor,
      type: "shape",
    },
  };
}

function shouldSkipShapePreview({
  fill,
  geometry,
  height,
  paragraphs,
  stroke,
  width,
}: {
  fill?: string;
  geometry: PresentationShapeGeometry;
  height: number;
  paragraphs: PresentationParagraph[];
  stroke?: string;
  width: number;
}): boolean {
  if (geometry === "line") {
    return false;
  }
  if (geometry === "unsupported" && paragraphs.length === 0) {
    return true;
  }
  if (!fill && !stroke && paragraphs.length === 0) {
    return true;
  }

  // 中文注释：PPT 里有些装饰点/图标会以复杂几何降级成小描边矩形。
  // 预览无法高保真还原时，隐藏它比显示误导性的半成品更接近系统预览体验。
  return (
    geometry === "rect" &&
    !fill &&
    !!stroke &&
    paragraphs.length === 0 &&
    Math.min(width, height) <= MIN_DECORATION_SHAPE_SIZE
  ) || (
    geometry === "roundRect" &&
    isPlainWhiteFill(fill) &&
    !stroke &&
    paragraphs.length === 0 &&
    Math.min(width, height) >= MIN_BACKGROUND_LIKE_SHAPE_SIZE
  );
}

function isPlainWhiteFill(fill?: string): boolean {
  const normalizedFill = fill?.toLowerCase();
  return normalizedFill === "#ffffff" || normalizedFill === "#fff";
}

async function parsePicture(
  zip: JSZip,
  slidePath: string,
  rels: Record<string, PresentationRelationship>,
  element: Element,
  id: string,
  objectUrls: string[],
): Promise<PresentationImageElement | null> {
  const shapeProperties = firstChildByLocalName(element, "spPr");
  const transform = readTransform(shapeProperties);
  const blip = firstDescendantByLocalName(element, "blip");
  const relId = blip ? relationshipAttribute(blip, "embed") || relationshipAttribute(blip, "link") : undefined;
  const rel = relId ? rels[relId] : undefined;

  if (!transform || !rel || rel.targetMode === "External") {
    return null;
  }

  const mediaPath = resolveRelationshipTarget(slidePath, rel.target);
  const mediaFile = zip.file(mediaPath);
  if (!mediaFile) {
    return null;
  }

  const blob = await mediaFile.async("blob");
  const src = URL.createObjectURL(blob);
  objectUrls.push(src);

  return {
    ...transform,
    id,
    src,
    type: "image",
  };
}

function readPlaceholderKey(element: Element): string | undefined {
  const placeholder = firstDescendantByLocalName(element, "ph");
  if (!placeholder) {
    return undefined;
  }

  const index = placeholder.getAttribute("idx");
  if (index) {
    return `idx:${index}`;
  }

  const type = placeholder.getAttribute("type") || "body";
  return `type:${type}`;
}

function readTextAnchor(bodyProperties: Element | null): PresentationShapeElement["textAnchor"] {
  const anchor = bodyProperties?.getAttribute("anchor");
  if (anchor === "ctr") {
    return "center";
  }
  if (anchor === "b") {
    return "bottom";
  }
  return "top";
}
