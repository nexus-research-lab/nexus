export interface WorkspaceFilePreviewProps {
  agentId: string;
  fileName: string;
  initialContent?: string | null;
  isPreviewFocused: boolean;
  onTogglePreviewFocus: () => void;
  path: string;
  showFocusControl?: boolean;
}
