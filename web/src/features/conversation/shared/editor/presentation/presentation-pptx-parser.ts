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
  type PresentationPlaceholderStyle,
  type PresentationRelationship,
  type PresentationShapeElement,
  type PresentationShapeGeometry,
  type PresentationSlide,
  type PresentationTransform,
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

interface PresentationSlideParts {
  layoutPart: PresentationPart | null;
  masterPart: PresentationPart | null;
  slideDoc: Document;
  slideRels: Record<string, PresentationRelationship>;
}

interface PresentationPart {
  background?: string;
  elements: PresentationElement[];
  placeholderStyles: Map<string, PresentationPlaceholderStyle>;
}

interface PresentationShapeTreeContext {
  elementIndex: number;
  fallbackPlaceholders?: Map<string, PresentationPlaceholderStyle>;
  idPrefix: string;
  includePlaceholderShapes: boolean;
}

interface PresentationShapeTreeResult {
  elements: PresentationElement[];
  placeholderStyles: Map<string, PresentationPlaceholderStyle>;
}

interface ShapeTreeParseInput {
  context: PresentationShapeTreeContext;
  objectUrls: string[];
  rels: Record<string, PresentationRelationship>;
  slidePath: string;
  state: PresentationShapeTreeResult;
  zip: JSZip;
}

type ShapeTreeChildHandler = (
  child: Element,
  input: ShapeTreeParseInput,
) => Promise<void> | void;

interface ParsedPresentationShape {
  isPlaceholder: boolean;
  placeholderStyle: PresentationPlaceholderStyle | null;
  shape: PresentationShapeElement | null;
}

interface ResolvedPresentationShape {
  fill?: string;
  geometry: PresentationShapeGeometry;
  paragraphs: PresentationParagraph[];
  stroke?: string;
  strokeWidth: number;
  textAnchor: PresentationShapeElement["textAnchor"];
  transform: PresentationTransform;
}

interface ShapePreviewRuleContext {
  fill?: string;
  geometry: PresentationShapeGeometry;
  height: number;
  paragraphs: PresentationParagraph[];
  stroke?: string;
  width: number;
}

type ShapeSkipRule = (context: ShapePreviewRuleContext) => boolean;

// 只隐藏能够明确识别的降级装饰或背景，未知组合仍交给预览层呈现。
const SHAPE_SKIP_RULES: ShapeSkipRule[] = [
  isUnsupportedEmptyShape,
  isInvisibleEmptyShape,
  isDecorationFallbackShape,
  isBackgroundFallbackShape,
];

const SHAPE_TREE_CHILD_HANDLERS: Record<string, ShapeTreeChildHandler> = {
  cxnSp: parseShapeTreeShapeChild,
  grpSp: parseShapeTreeGroupChild,
  pic: parseShapeTreePictureChild,
  sp: parseShapeTreeShapeChild,
};

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
  const parts = await readPresentationSlideParts(zip, slidePath, objectUrls);
  const inheritedPlaceholders = mergePlaceholderStyles(
    parts.masterPart?.placeholderStyles,
    parts.layoutPart?.placeholderStyles,
  );
  const slideResult = await parseSlideShapeTree(
    zip,
    slidePath,
    parts,
    objectUrls,
    index,
    inheritedPlaceholders,
  );

  return {
    background: resolveSlideBackground(parts),
    elements: collectSlideElements(parts, slideResult),
    height,
    id: `slide-${index + 1}`,
    title: readSlideTitle(slideResult.elements, index),
    width,
  };
}

async function readPresentationSlideParts(
  zip: JSZip,
  slidePath: string,
  objectUrls: string[],
): Promise<PresentationSlideParts> {
  const slideXml = await readZipText(zip, slidePath);
  const slideDoc = parseXml(slideXml);
  const slideRels = await readRelationships(zip, slidePath);
  const layoutPath = resolveRelatedPartPath(slidePath, slideRels, SLIDE_LAYOUT_RELATIONSHIP_TYPE);
  const layoutRels = await readOptionalRelationships(zip, layoutPath);
  const masterPath = resolveRelatedPartPath(
    layoutPath ?? "",
    layoutRels,
    SLIDE_MASTER_RELATIONSHIP_TYPE,
  );
  const masterPart = await parseOptionalPresentationPart(
    zip,
    masterPath,
    objectUrls,
  );
  const layoutPart = await parseOptionalPresentationPart(
    zip,
    layoutPath,
    objectUrls,
    masterPart?.placeholderStyles,
  );
  return { layoutPart, masterPart, slideDoc, slideRels };
}

async function readOptionalRelationships(
  zip: JSZip,
  partPath: string | null,
): Promise<Record<string, PresentationRelationship>> {
  return partPath ? readRelationships(zip, partPath) : {};
}

async function parseOptionalPresentationPart(
  zip: JSZip,
  partPath: string | null,
  objectUrls: string[],
  fallbackPlaceholders?: Map<string, PresentationPlaceholderStyle>,
): Promise<PresentationPart | null> {
  return partPath
    ? parsePresentationPart(zip, partPath, objectUrls, fallbackPlaceholders)
    : null;
}

async function parseSlideShapeTree(
  zip: JSZip,
  slidePath: string,
  parts: PresentationSlideParts,
  objectUrls: string[],
  index: number,
  inheritedPlaceholders: Map<string, PresentationPlaceholderStyle>,
): Promise<PresentationShapeTreeResult> {
  const shapeTree = firstDescendantByLocalName(parts.slideDoc, "spTree");
  if (!shapeTree) {
    return emptyShapeTreeResult();
  }
  return parseShapeTree(
    zip,
    slidePath,
    parts.slideRels,
    shapeTree,
    objectUrls,
    {
      elementIndex: 0,
      fallbackPlaceholders: inheritedPlaceholders,
      idPrefix: `slide-${index + 1}`,
      includePlaceholderShapes: true,
    },
  );
}

function emptyShapeTreeResult(): PresentationShapeTreeResult {
  return {
    elements: [],
    placeholderStyles: new Map<string, PresentationPlaceholderStyle>(),
  };
}

function resolveSlideBackground(parts: PresentationSlideParts): string {
  return readSlideBackground(parts.slideDoc)
    ?? parts.layoutPart?.background
    ?? parts.masterPart?.background
    ?? "#ffffff";
}

function collectSlideElements(
  parts: PresentationSlideParts,
  slideResult: PresentationShapeTreeResult,
): PresentationElement[] {
  return [
    ...readPartElements(parts.masterPart),
    ...readPartElements(parts.layoutPart),
    ...slideResult.elements,
  ];
}

function readPartElements(part: PresentationPart | null): PresentationElement[] {
  return part?.elements ?? [];
}

function readSlideTitle(
  elements: PresentationElement[],
  index: number,
): string {
  const firstText = elements
    .flatMap((element) => element.type === "shape" ? element.paragraphs : [])
    .map((paragraph) => paragraph.text.trim())
    .find(Boolean);
  return firstText ?? `幻灯片 ${index + 1}`;
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
  }) : emptyShapeTreeResult();

  return {
    background: readSlideBackground(partDoc),
    elements: result.elements,
    placeholderStyles: result.placeholderStyles,
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
  const state = emptyShapeTreeResult();
  const input: ShapeTreeParseInput = {
    context,
    objectUrls,
    rels,
    slidePath,
    state,
    zip,
  };

  for (const child of Array.from(shapeTree.children)) {
    await SHAPE_TREE_CHILD_HANDLERS[child.localName]?.(child, input);
  }

  return applyShapeTreeGroupTransform(state, groupTransform);
}

function parseShapeTreeShapeChild(
  child: Element,
  input: ShapeTreeParseInput,
): void {
  const parsedShape = parseShape(
    child,
    consumeShapeTreeElementId(input.context, "shape"),
    input.context,
  );
  registerPlaceholderStyle(input.state, parsedShape.placeholderStyle);
  if (isVisibleParsedShape(parsedShape, input.context)) {
    input.state.elements.push(parsedShape.shape);
  }
}

async function parseShapeTreeGroupChild(
  child: Element,
  input: ShapeTreeParseInput,
): Promise<void> {
  const groupResult = await parseShapeTree(
    input.zip,
    input.slidePath,
    input.rels,
    child,
    input.objectUrls,
    input.context,
    readGroupTransform(child),
  );
  mergeShapeTreeResult(input.state, groupResult);
}

async function parseShapeTreePictureChild(
  child: Element,
  input: ShapeTreeParseInput,
): Promise<void> {
  const image = await parsePicture(
    input.zip,
    input.slidePath,
    input.rels,
    child,
    consumeShapeTreeElementId(input.context, "image"),
    input.objectUrls,
  );
  if (image) {
    input.state.elements.push(image);
  }
}

function consumeShapeTreeElementId(
  context: PresentationShapeTreeContext,
  kind: "image" | "shape",
): string {
  const id = `${context.idPrefix}-${kind}-${context.elementIndex}`;
  context.elementIndex += 1;
  return id;
}

function registerPlaceholderStyle(
  state: PresentationShapeTreeResult,
  style: PresentationPlaceholderStyle | null,
): void {
  if (style) {
    state.placeholderStyles.set(style.key, style);
  }
}

function isVisibleParsedShape(
  parsedShape: ParsedPresentationShape,
  context: PresentationShapeTreeContext,
): parsedShape is ParsedPresentationShape & { shape: PresentationShapeElement } {
  return parsedShape.shape !== null
    && (!parsedShape.isPlaceholder || context.includePlaceholderShapes);
}

function mergeShapeTreeResult(
  target: PresentationShapeTreeResult,
  source: PresentationShapeTreeResult,
): void {
  target.elements.push(...source.elements);
  source.placeholderStyles.forEach((style, key) => {
    target.placeholderStyles.set(key, style);
  });
}

function applyShapeTreeGroupTransform(
  result: PresentationShapeTreeResult,
  groupTransform?: PresentationGroupTransform | null,
): PresentationShapeTreeResult {
  if (!groupTransform) {
    return result;
  }

  return {
    elements: result.elements.map((element) => (
      applyGroupTransformToElement(element, groupTransform)
    )),
    placeholderStyles: mapGroupPlaceholderStyles(
      result.placeholderStyles,
      groupTransform,
    ),
  };
}

function parseShape(
  element: Element,
  id: string,
  context: PresentationShapeTreeContext,
): ParsedPresentationShape {
  const shapeProperties = firstChildByLocalName(element, "spPr");
  const placeholderKey = readPlaceholderKey(element);
  const fallbackPlaceholder = resolveFallbackPlaceholder(
    context,
    placeholderKey,
  );
  const resolvedShape = resolvePresentationShape(
    element,
    shapeProperties,
    fallbackPlaceholder,
  );
  const isPlaceholder = placeholderKey !== undefined;
  if (!resolvedShape) {
    return { isPlaceholder, placeholderStyle: null, shape: null };
  }

  const placeholderStyle = createPlaceholderStyle(
    placeholderKey,
    resolvedShape,
  );
  const shape = shouldSkipShapePreview(resolvedShape)
    ? null
    : createShapeElement(id, resolvedShape);
  return { isPlaceholder, placeholderStyle, shape };
}

function resolveFallbackPlaceholder(
  context: PresentationShapeTreeContext,
  placeholderKey?: string,
): PresentationPlaceholderStyle | undefined {
  return placeholderKey
    ? context.fallbackPlaceholders?.get(placeholderKey)
    : undefined;
}

function resolvePresentationShape(
  element: Element,
  shapeProperties: Element | null,
  fallbackPlaceholder?: PresentationPlaceholderStyle,
): ResolvedPresentationShape | null {
  const transform = resolveShapeTransform(shapeProperties, fallbackPlaceholder);
  if (!transform) {
    return null;
  }

  const textBody = firstChildByLocalName(element, "txBody");
  return {
    fill: resolveShapeFill(shapeProperties, fallbackPlaceholder),
    geometry: resolveShapeGeometry(
      element,
      shapeProperties,
      fallbackPlaceholder,
    ),
    paragraphs: parseTextBody(textBody, transform.width),
    stroke: resolveShapeStroke(shapeProperties, fallbackPlaceholder),
    strokeWidth: resolveShapeStrokeWidth(shapeProperties, fallbackPlaceholder),
    textAnchor: readTextAnchor(firstChildByLocalName(textBody, "bodyPr")),
    transform,
  };
}

function resolveShapeTransform(
  shapeProperties: Element | null,
  fallbackPlaceholder?: PresentationPlaceholderStyle,
): PresentationTransform | null {
  return readTransform(shapeProperties)
    ?? fallbackPlaceholder?.transform
    ?? null;
}

function resolveShapeFill(
  shapeProperties: Element | null,
  fallbackPlaceholder?: PresentationPlaceholderStyle,
): string | undefined {
  return readFillColor(shapeProperties) ?? fallbackPlaceholder?.fill;
}

function resolveShapeStroke(
  shapeProperties: Element | null,
  fallbackPlaceholder?: PresentationPlaceholderStyle,
): string | undefined {
  return readStrokeColor(shapeProperties) ?? fallbackPlaceholder?.stroke;
}

function resolveShapeStrokeWidth(
  shapeProperties: Element | null,
  fallbackPlaceholder?: PresentationPlaceholderStyle,
): number {
  return readStrokeWidth(shapeProperties)
    ?? fallbackPlaceholder?.strokeWidth
    ?? 1;
}

function resolveShapeGeometry(
  element: Element,
  shapeProperties: Element | null,
  fallbackPlaceholder?: PresentationPlaceholderStyle,
): PresentationShapeGeometry {
  return readShapeGeometry(
    shapeProperties,
    element.localName === "cxnSp",
    fallbackPlaceholder?.geometry,
  );
}

function createPlaceholderStyle(
  placeholderKey: string | undefined,
  resolvedShape: ResolvedPresentationShape,
): PresentationPlaceholderStyle | null {
  if (!placeholderKey) {
    return null;
  }
  return {
    fill: resolvedShape.fill,
    geometry: resolvedShape.geometry,
    key: placeholderKey,
    stroke: resolvedShape.stroke,
    strokeWidth: resolvedShape.strokeWidth,
    transform: resolvedShape.transform,
  };
}

function createShapeElement(
  id: string,
  resolvedShape: ResolvedPresentationShape,
): PresentationShapeElement {
  return {
    ...resolvedShape.transform,
    fill: resolvedShape.fill,
    geometry: resolvedShape.geometry,
    id,
    paragraphs: resolvedShape.paragraphs,
    stroke: resolvedShape.stroke,
    strokeWidth: resolvedShape.strokeWidth,
    textAnchor: resolvedShape.textAnchor,
    type: "shape",
  };
}

function shouldSkipShapePreview(
  resolvedShape: ResolvedPresentationShape,
): boolean {
  const context = createShapePreviewRuleContext(resolvedShape);
  if (context.geometry === "line") {
    return false;
  }
  return SHAPE_SKIP_RULES.some((rule) => rule(context));
}

function createShapePreviewRuleContext(
  resolvedShape: ResolvedPresentationShape,
): ShapePreviewRuleContext {
  return {
    fill: resolvedShape.fill,
    geometry: resolvedShape.geometry,
    height: resolvedShape.transform.height,
    paragraphs: resolvedShape.paragraphs,
    stroke: resolvedShape.stroke,
    width: resolvedShape.transform.width,
  };
}

function isUnsupportedEmptyShape(context: ShapePreviewRuleContext): boolean {
  return context.geometry === "unsupported" && hasNoShapeText(context);
}

function isInvisibleEmptyShape(context: ShapePreviewRuleContext): boolean {
  return !context.fill && !context.stroke && hasNoShapeText(context);
}

function isDecorationFallbackShape(context: ShapePreviewRuleContext): boolean {
  return context.geometry === "rect"
    && !context.fill
    && !!context.stroke
    && hasNoShapeText(context)
    && Math.min(context.width, context.height) <= MIN_DECORATION_SHAPE_SIZE;
}

function isBackgroundFallbackShape(context: ShapePreviewRuleContext): boolean {
  return context.geometry === "roundRect"
    && isPlainWhiteFill(context.fill)
    && !context.stroke
    && hasNoShapeText(context)
    && Math.min(context.width, context.height) >= MIN_BACKGROUND_LIKE_SHAPE_SIZE;
}

function hasNoShapeText(context: ShapePreviewRuleContext): boolean {
  return context.paragraphs.length === 0;
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
