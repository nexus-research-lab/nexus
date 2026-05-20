"use client";

import { useEffect, useMemo, useState } from "react";
import DOMPurify from "dompurify";
import { FileWarning, LoaderCircle } from "lucide-react";

import {
  get_workspace_file_content_api,
  get_workspace_file_preview_url,
} from "@/lib/api/agent-manage-api";
import { cn } from "@/lib/utils";
import { MarkdownRendererContent } from "@/features/conversation/shared/message/markdown/markdown-renderer-content";
import { CodeBlock } from "@/features/conversation/shared/message/blocks/code-block";
import { FileArtifactBlock } from "@/features/conversation/shared/message/blocks/file-artifact-block";
import type { ArtifactItem } from "@/types/conversation/artifact";

import { MermaidArtifactView } from "./mermaid-artifact-view";

interface ArtifactPreviewProps {
  artifact: ArtifactItem | null;
  class_name?: string;
  on_open_workspace_file?: (path: string) => void;
}

function should_fetch_artifact_content(artifact: ArtifactItem): boolean {
  return Boolean(
    artifact.path &&
    artifact.workspace_agent_id &&
    !artifact.content &&
    ["markdown", "html", "mermaid", "code"].includes(artifact.kind),
  );
}

function language_for_artifact(artifact: ArtifactItem): string {
  if (artifact.language) {
    return artifact.language;
  }
  if (artifact.kind === "html") {
    return "html";
  }
  if (artifact.kind === "mermaid") {
    return "mermaid";
  }
  if (artifact.kind === "markdown") {
    return "markdown";
  }
  return "text";
}

function EmptyPreview() {
  return (
    <div className="flex h-full min-h-0 items-center justify-center px-8 text-center text-(--text-muted)">
      <div className="max-w-sm">
        <p className="text-sm font-semibold uppercase tracking-[0.18em]">Artifacts</p>
        <p className="mt-3 text-sm leading-6">
          当前会话还没有可预览的产物。生成图片、HTML、流程图或文件后，会在这里集中展示。
        </p>
      </div>
    </div>
  );
}

function PreviewError({ message }: { message: string }) {
  return (
    <div className="m-auto max-w-sm rounded-[8px] border border-destructive/20 bg-destructive/6 px-4 py-3 text-sm text-destructive">
      <div className="flex items-center gap-2 font-medium">
        <FileWarning className="h-4 w-4" />
        产物预览失败
      </div>
      <p className="mt-2 leading-6">{message}</p>
    </div>
  );
}

export function ArtifactPreview({ artifact, class_name, on_open_workspace_file }: ArtifactPreviewProps) {
  const [content, set_content] = useState<string | null>(null);
  const [is_loading, set_is_loading] = useState(false);
  const [error, set_error] = useState<string | null>(null);

  useEffect(() => {
    let ignore = false;
    set_error(null);
    set_content(artifact?.content ?? null);

    if (!artifact || !should_fetch_artifact_content(artifact)) {
      set_is_loading(false);
      return;
    }

    const load_content = async () => {
      set_is_loading(true);
      try {
        const response = await get_workspace_file_content_api(artifact.workspace_agent_id!, artifact.path!);
        if (!ignore) {
          set_content(response.content);
        }
      } catch (load_error) {
        if (!ignore) {
          set_error(load_error instanceof Error ? load_error.message : "读取产物失败");
        }
      } finally {
        if (!ignore) {
          set_is_loading(false);
        }
      }
    };

    void load_content();
    return () => {
      ignore = true;
    };
  }, [artifact]);

  const preview_url = useMemo(() => {
    if (!artifact?.path || !artifact.workspace_agent_id) {
      return artifact?.url ?? "";
    }
    return get_workspace_file_preview_url(artifact.workspace_agent_id, artifact.path);
  }, [artifact]);

  const sanitized_html = useMemo(() => {
    if (artifact?.kind !== "html" || !content) {
      return "";
    }
    return DOMPurify.sanitize(content, { WHOLE_DOCUMENT: true });
  }, [artifact?.kind, content]);

  if (!artifact) {
    return <EmptyPreview />;
  }

  if (is_loading) {
    return (
      <div className={cn("flex h-full items-center justify-center text-sm text-(--text-muted)", class_name)}>
        <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
        正在加载产物
      </div>
    );
  }

  if (error) {
    return (
      <div className={cn("flex h-full items-center justify-center p-6", class_name)}>
        <PreviewError message={error} />
      </div>
    );
  }

  if ((artifact.kind === "image" || artifact.kind === "svg") && preview_url) {
    return (
      <div className={cn("flex h-full min-h-0 items-center justify-center overflow-auto bg-[var(--surface-panel-subtle-background)] p-6", class_name)}>
        <img
          alt={artifact.title}
          className="max-h-full max-w-full rounded-[8px] object-contain"
          src={preview_url}
        />
      </div>
    );
  }

  if (artifact.kind === "pdf" && preview_url) {
    return (
      <iframe
        className={cn("h-full w-full border-0 bg-[var(--surface-panel-subtle-background)]", class_name)}
        src={preview_url}
        title={artifact.title}
      />
    );
  }

  if (artifact.kind === "markdown") {
    return (
      <div className={cn("soft-scrollbar h-full overflow-auto px-5 py-4", class_name)}>
        <MarkdownRendererContent content={content ?? artifact.content ?? ""} />
      </div>
    );
  }

  if (artifact.kind === "html") {
    return (
      <iframe
        className={cn("h-full w-full border-0 bg-white", class_name)}
        sandbox=""
        srcDoc={sanitized_html}
        title={artifact.title}
      />
    );
  }

  if (artifact.kind === "mermaid") {
    return (
      <div className={cn("h-full min-h-0 overflow-hidden p-4", class_name)}>
        <MermaidArtifactView chart={content ?? artifact.content ?? ""} />
      </div>
    );
  }

  if (content || artifact.content) {
    return (
      <div className={cn("soft-scrollbar h-full overflow-auto p-4", class_name)}>
        <CodeBlock language={language_for_artifact(artifact)} value={content ?? artifact.content ?? ""} />
      </div>
    );
  }

  return (
    <div className={cn("flex h-full items-center justify-center p-6", class_name)}>
      {artifact.path ? (
        <FileArtifactBlock
          label="工作区文件"
          path={artifact.path}
          display_path={artifact.display_path ?? artifact.path}
          on_open_workspace_file={on_open_workspace_file}
        />
      ) : (
        <PreviewError message="这个产物没有可预览内容。" />
      )}
    </div>
  );
}
