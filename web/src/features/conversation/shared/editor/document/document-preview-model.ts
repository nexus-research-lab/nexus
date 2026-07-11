import type { Options as DocxPreviewOptions } from "docx-preview";

export type DocumentPreviewStatus =
  | { state: "loading"; message: string }
  | { state: "loaded" }
  | { state: "error"; message: string };

export const DOCX_RENDER_OPTIONS: Partial<DocxPreviewOptions> = {
  breakPages: true,
  className: "nexus-docx-preview",
  debug: false,
  experimental: false,
  ignoreFonts: false,
  ignoreHeight: false,
  ignoreLastRenderedPageBreak: false,
  ignoreWidth: false,
  inWrapper: true,
  renderAltChunks: false,
  renderChanges: false,
  renderComments: false,
  renderEndnotes: true,
  renderFooters: true,
  renderFootnotes: true,
  renderHeaders: true,
  trimXmlDeclaration: true,
  useBase64URL: true,
};
