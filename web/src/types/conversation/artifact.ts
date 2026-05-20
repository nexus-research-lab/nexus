export type ArtifactKind =
  | "image"
  | "markdown"
  | "html"
  | "mermaid"
  | "svg"
  | "pdf"
  | "file"
  | "code"
  | "unknown";

export type ArtifactSource =
  | "workspace_file_artifact"
  | "markdown_image"
  | "markdown_path"
  | "image_block"
  | "code_fence";

export interface ArtifactItem {
  id: string;
  kind: ArtifactKind;
  source: ArtifactSource;
  title: string;
  path?: string | null;
  display_path?: string | null;
  workspace_agent_id?: string | null;
  mime_type?: string | null;
  content?: string | null;
  url?: string | null;
  language?: string | null;
  message_id?: string | null;
  round_id?: string | null;
  created_at: number;
}
