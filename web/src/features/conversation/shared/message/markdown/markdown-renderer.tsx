"use client";

import { cn } from "@/lib/utils";
import ReactMarkdown from "react-markdown";

import "katex/dist/katex.min.css";

import {
  create_markdown_components,
  MARKDOWN_BODY_CLASS_NAME,
  MARKDOWN_PLUGINS,
  REHYPE_PLUGINS,
  split_markdown_file_artifacts,
  useMarkdownFileResolver,
} from "./markdown-renderer-shared";
import { FileArtifactBlock } from "../blocks/file-artifact-block";

interface MarkdownRendererProps {
  content: string;
  class_name?: string;
  is_streaming?: boolean;
  on_open_workspace_file?: (path: string) => void;
}

export function MarkdownRenderer(props: MarkdownRendererProps) {
  const { content, class_name, is_streaming, on_open_workspace_file } = props;
  const resolve_file_path = useMarkdownFileResolver();
  const markdown_components = create_markdown_components(resolve_file_path, on_open_workspace_file);
  const content_segments = on_open_workspace_file
    ? split_markdown_file_artifacts(content, resolve_file_path)
    : [{ type: "text" as const, text: content }];

  return (
    <div
      className={cn(
        MARKDOWN_BODY_CLASS_NAME,
        is_streaming && "animate-in fade-in-0",
        class_name,
      )}
    >
      {content_segments.map((segment, index) => {
        if (segment.type === "file_artifact") {
          return (
            <FileArtifactBlock
              key={`file-artifact-${index}-${segment.path}`}
              label={segment.label}
              path={segment.path}
              display_path={segment.display_path}
              on_open_workspace_file={on_open_workspace_file}
            />
          );
        }

        if (!segment.text.trim()) {
          return null;
        }

        return (
          <ReactMarkdown
            key={`text-${index}`}
            components={markdown_components}
            rehypePlugins={REHYPE_PLUGINS}
            remarkPlugins={MARKDOWN_PLUGINS}
          >
            {segment.text}
          </ReactMarkdown>
        );
      })}
    </div>
  );
}
