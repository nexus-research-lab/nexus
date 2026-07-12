import type JSZip from "jszip";

import {
  MIN_BACKGROUND_LIKE_SHAPE_SIZE,
  MIN_DECORATION_SHAPE_SIZE,
  type PresentationElement,
  type PresentationGroupTransform,
  type PresentationImageElement,
  type PresentationParagraph,
  type PresentationPlaceholderStyle,
  type PresentationRelationship,
  type PresentationShapeElement,
  type PresentationShapeGeometry,
  type PresentationTransform,
} from "../presentation-preview-model";
import {
  applyGroupTransformToElement,
  mapGroupPlaceholderStyles,
  readFillColor,
  readGroupTransform,
  readShapeGeometry,
  readStrokeColor,
  readStrokeWidth,
  readTransform,
} from "../presentation-shape-style";
import { parseTextBody } from "../presentation-text-parser";
import {
  firstChildByLocalName,
  firstDescendantByLocalName,
  relationshipAttribute,
  resolveRelationshipTarget,
} from "../presentation-xml-utils";

interface PresentationShapeTreeContext {
  elementIndex: number;
  fallbackPlaceholders?: Map<string, PresentationPlaceholderStyle>;
  idPrefix: string;
  includePlaceholderShapes: boolean;
}

export interface PresentationShapeTreeResult {
  elements: PresentationElement[];
  placeholderStyles: Map<string, PresentationPlaceholderStyle>;
}

interface ParsePresentationShapeTreeOptions {
  context: PresentationShapeTreeContext;
  groupTransform?: PresentationGroupTransform | null;
  objectUrls: string[];
  relationships: Record<string, PresentationRelationship>;
  shapeTree: Element;
  sourcePath: string;
  zip: JSZip;
}

interface ShapeTreeParseInput {
  context: PresentationShapeTreeContext;
  objectUrls: string[];
  relationships: Record<string, PresentationRelationship>;
  sourcePath: string;
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

export async function parsePresentationShapeTree({
  context,
  groupTransform,
  objectUrls,
  relationships,
  shapeTree,
  sourcePath,
  zip,
}: ParsePresentationShapeTreeOptions): Promise<PresentationShapeTreeResult> {
  const state = createEmptyPresentationShapeTreeResult();
  const input: ShapeTreeParseInput = {
    context,
    objectUrls,
    relationships,
    sourcePath,
    state,
    zip,
  };
  for (const child of Array.from(shapeTree.children)) {
    await SHAPE_TREE_CHILD_HANDLERS[child.localName]?.(child, input);
  }
  return applyShapeTreeGroupTransform(state, groupTransform);
}

export function createEmptyPresentationShapeTreeResult(): PresentationShapeTreeResult {
  return {
    elements: [],
    placeholderStyles: new Map<string, PresentationPlaceholderStyle>(),
  };
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
  const groupResult = await parsePresentationShapeTree({
    context: input.context,
    groupTransform: readGroupTransform(child),
    objectUrls: input.objectUrls,
    relationships: input.relationships,
    shapeTree: child,
    sourcePath: input.sourcePath,
    zip: input.zip,
  });
  mergeShapeTreeResult(input.state, groupResult);
}

async function parseShapeTreePictureChild(
  child: Element,
  input: ShapeTreeParseInput,
): Promise<void> {
  const image = await parsePicture(
    input.zip,
    input.sourcePath,
    input.relationships,
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
    && Boolean(context.stroke)
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
  sourcePath: string,
  relationships: Record<string, PresentationRelationship>,
  element: Element,
  id: string,
  objectUrls: string[],
): Promise<PresentationImageElement | null> {
  const shapeProperties = firstChildByLocalName(element, "spPr");
  const transform = readTransform(shapeProperties);
  const blip = firstDescendantByLocalName(element, "blip");
  const relationshipId = blip
    ? relationshipAttribute(blip, "embed")
      || relationshipAttribute(blip, "link")
    : undefined;
  const relationship = relationshipId
    ? relationships[relationshipId]
    : undefined;
  if (!transform || !relationship || relationship.targetMode === "External") {
    return null;
  }
  const mediaPath = resolveRelationshipTarget(
    sourcePath,
    relationship.target,
  );
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

function readTextAnchor(
  bodyProperties: Element | null,
): PresentationShapeElement["textAnchor"] {
  const anchor = bodyProperties?.getAttribute("anchor");
  if (anchor === "ctr") {
    return "center";
  }
  if (anchor === "b") {
    return "bottom";
  }
  return "top";
}
