export type WorkspaceTextView = "rendered" | "source";

export interface WorkspaceSourceFocus {
  endLine?: number | null;
  snippets?: string[];
  startLine?: number | null;
  tone: "read" | "write" | "edit" | "error";
}

export interface WorkspaceFilePreviewProps {
  agentId: string;
  fileName: string;
  initialContent?: string | null;
  isPreviewFocused: boolean;
  onTogglePreviewFocus: () => void;
  path: string;
  showFocusControl?: boolean;
  sourceFocus?: WorkspaceSourceFocus | null;
  textView?: WorkspaceTextView;
}
