// INPUT: 已分类的文件类型、精确文件身份与预览交互参数。
// OUTPUT: 稳定渲染表分派的直接或懒加载预览；Office 复用同一加载外壳。
// POS: 文件预览类型路由，不拥有文件 IO、布局状态或业务写命令。

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

const PREVIEW_RENDERERS: Record<WorkspaceFilePreviewKind, PreviewRenderer> = {
  binary: createDirectPreviewRenderer(BinaryFilePlaceholder),
  document: createOfficePreviewRenderer("document", DocumentFilePreview),
  html: TextPreviewRenderer,
  image: createDirectPreviewRenderer(ImagePreview),
  markdown: TextPreviewRenderer,
  mermaid: TextPreviewRenderer,
  pdf: createDirectPreviewRenderer(PdfPreview),
  presentation: createOfficePreviewRenderer(
    "presentation",
    PresentationFilePreview,
  ),
  spreadsheet: createOfficePreviewRenderer(
    "spreadsheet",
    SpreadsheetFilePreview,
  ),
  text: TextPreviewRenderer,
};

/** 文件类型路由由完整描述表维护，新增预览类型不再修改面板分支。 */
export function WorkspaceFilePreviewRouter(
  props: WorkspaceFilePreviewRouterProps,
) {
  const Renderer = PREVIEW_RENDERERS[props.fileType];
  return <Renderer {...props} />;
}
