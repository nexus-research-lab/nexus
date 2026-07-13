import type { StageWindowKind } from "../operation-desktop-types";

export type OperationAppSurfaceKind = "document" | "specialized";

export function appSurfaceForWindowKind(kind: StageWindowKind): OperationAppSurfaceKind {
  if (
    kind === "code_editor" ||
    kind === "markdown_reader" ||
    kind === "word_reader" ||
    kind === "pdf_reader" ||
    kind === "spreadsheet" ||
    kind === "image_viewer"
  ) {
    return "document";
  }
  return "specialized";
}
