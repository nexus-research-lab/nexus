export const MAX_PPTX_PREVIEW_BYTES = 15 * 1024 * 1024;
export const EMU_PER_PIXEL = 9525;
export const DEFAULT_SLIDE_WIDTH_EMU = 12192000;
export const DEFAULT_SLIDE_HEIGHT_EMU = 6858000;
export const RELATIONSHIP_NAMESPACE = "http://schemas.openxmlformats.org/officeDocument/2006/relationships";
export const SLIDE_LAYOUT_RELATIONSHIP_TYPE = `${RELATIONSHIP_NAMESPACE}/slideLayout`;
export const SLIDE_MASTER_RELATIONSHIP_TYPE = `${RELATIONSHIP_NAMESPACE}/slideMaster`;
export const ROUND_RECT_RADIUS_RATIO = 0.08;
export const ROUND_RECT_MAX_RADIUS = 14;
export const MIN_DECORATION_SHAPE_SIZE = 28;
export const MIN_BACKGROUND_LIKE_SHAPE_SIZE = 240;

export const SCHEME_COLORS: Record<string, string> = {
  accent1: "#4472c4",
  accent2: "#ed7d31",
  accent3: "#a5a5a5",
  accent4: "#ffc000",
  accent5: "#5b9bd5",
  accent6: "#70ad47",
  bg1: "#ffffff",
  bg2: "#f2f2f2",
  dk1: "#111827",
  dk2: "#1f2937",
  lt1: "#ffffff",
  lt2: "#f8fafc",
  tx1: "#111827",
  tx2: "#374151",
};

export type PresentationPreviewStatus =
  | { state: "loading"; message: string }
  | { state: "loaded"; slide_count: number }
  | { state: "error"; message: string };

export type PresentationShapeGeometry =
  | "diamond"
  | "ellipse"
  | "line"
  | "rect"
  | "roundRect"
  | "triangle"
  | "unsupported";

export interface PresentationTextRun {
  bold?: boolean;
  color?: string;
  font_face?: string;
  font_size: number;
  italic?: boolean;
  text: string;
}

export interface PresentationParagraph {
  align?: "center" | "left" | "right";
  bullet?: string;
  bullet_indent: number;
  font_size: number;
  line_height: number;
  runs: PresentationTextRun[];
  text: string;
}

export interface PresentationShapeElement {
  fill?: string;
  geometry: PresentationShapeGeometry;
  height: number;
  id: string;
  paragraphs: PresentationParagraph[];
  stroke?: string;
  stroke_width: number;
  text_anchor: "bottom" | "center" | "top";
  type: "shape";
  width: number;
  x: number;
  y: number;
}

export interface PresentationImageElement {
  height: number;
  id: string;
  src: string;
  type: "image";
  width: number;
  x: number;
  y: number;
}

export type PresentationElement = PresentationImageElement | PresentationShapeElement;

export interface PresentationSlide {
  background: string;
  elements: PresentationElement[];
  height: number;
  id: string;
  title: string;
  width: number;
}

export interface PresentationRelationship {
  target: string;
  target_mode?: string;
  type?: string;
}

export interface PresentationParseResult {
  object_urls: string[];
  slides: PresentationSlide[];
}

export type PresentationTransform = Pick<PresentationShapeElement, "height" | "width" | "x" | "y">;

export interface PresentationPlaceholderStyle {
  fill?: string;
  geometry: PresentationShapeGeometry;
  key: string;
  stroke?: string;
  stroke_width: number;
  transform: PresentationTransform;
}

export interface PresentationPart {
  background?: string;
  elements: PresentationElement[];
  placeholder_styles: Map<string, PresentationPlaceholderStyle>;
  rels: Record<string, PresentationRelationship>;
}

export interface PresentationShapeTreeContext {
  element_index: number;
  fallback_placeholders?: Map<string, PresentationPlaceholderStyle>;
  id_prefix: string;
  include_placeholder_shapes: boolean;
}

export interface PresentationShapeTreeResult {
  elements: PresentationElement[];
  placeholder_styles: Map<string, PresentationPlaceholderStyle>;
}

export interface PresentationGroupTransform extends PresentationTransform {
  child_height: number;
  child_width: number;
  child_x: number;
  child_y: number;
}
