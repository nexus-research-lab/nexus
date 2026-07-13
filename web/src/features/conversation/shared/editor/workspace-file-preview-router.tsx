import {
  lazy,
  Suspense,
  type ComponentType,
  type LazyExoticComponent,
} from "react";

import {
  BinaryFilePlaceholder,
  ImagePreview,
  PdfPreview,
} from "./media/media-file-preview";
import {
  OfficePreviewFallback,
  type OfficePreviewKind,
} from "./office-preview-fallbacks";
import { TextFileEditor } from "./text/text-file-editor";
import type { WorkspaceFilePreviewKind } from "./workspace-file-preview-kind";
import type { WorkspaceFilePreviewProps } from "./workspace-file-preview-types";

const SpreadsheetFilePreview = lazy(() => (
  import("./spreadsheet/spreadsheet-file-preview").then((module) => ({
    default: module.SpreadsheetFilePreview,
  }))
));
const DocumentFilePreview = lazy(() => (
  import("./document/document-file-preview").then((module) => ({
    default: module.DocumentFilePreview,
  }))
));
const PresentationFilePreview = lazy(() => (
  import("./presentation/presentation-file-preview").then((module) => ({
    default: module.PresentationFilePreview,
  }))
));

interface WorkspaceFilePreviewRouterProps extends WorkspaceFilePreviewProps {
  fileType: WorkspaceFilePreviewKind;
}

type PreviewRenderer = ComponentType<WorkspaceFilePreviewRouterProps>;

function TextPreviewRenderer({
  fileType,
  ...props
}: WorkspaceFilePreviewRouterProps) {
  return <TextFileEditor {...props} fileType={fileType} />;
}

function createDirectPreviewRenderer(
  Component: ComponentType<WorkspaceFilePreviewProps>,
): PreviewRenderer {
  return function DirectPreviewRenderer({
    fileType: _fileType,
    ...props
  }: WorkspaceFilePreviewRouterProps) {
    return <Component {...props} />;
  };
}

function createOfficePreviewRenderer(
  kind: OfficePreviewKind,
  Component: LazyExoticComponent<ComponentType<WorkspaceFilePreviewProps>>,
): PreviewRenderer {
  return function OfficePreviewRenderer({
    fileType: _fileType,
    ...props
  }: WorkspaceFilePreviewRouterProps) {
    return (
      <Suspense fallback={<OfficePreviewFallback {...props} kind={kind} />}>
        <Component {...props} />
      </Suspense>
    );
  };
}

const TEXT_PREVIEW_RENDERER = TextPreviewRenderer;
const PREVIEW_RENDERERS: Record<WorkspaceFilePreviewKind, PreviewRenderer> = {
  binary: createDirectPreviewRenderer(BinaryFilePlaceholder),
  document: createOfficePreviewRenderer("document", DocumentFilePreview),
  html: TEXT_PREVIEW_RENDERER,
  image: createDirectPreviewRenderer(ImagePreview),
  markdown: TEXT_PREVIEW_RENDERER,
  mermaid: TEXT_PREVIEW_RENDERER,
  pdf: createDirectPreviewRenderer(PdfPreview),
  presentation: createOfficePreviewRenderer(
    "presentation",
    PresentationFilePreview,
  ),
  spreadsheet: createOfficePreviewRenderer(
    "spreadsheet",
    SpreadsheetFilePreview,
  ),
  text: TEXT_PREVIEW_RENDERER,
};

/** 文件类型路由由完整描述表维护，新增预览类型不再修改面板分支。 */
export function WorkspaceFilePreviewRouter(
  props: WorkspaceFilePreviewRouterProps,
) {
  const Renderer = PREVIEW_RENDERERS[props.fileType];
  return <Renderer {...props} />;
}
