export interface WorkspaceFilePreviewProps {
  agentId: string;
  fileName: string;
  isPreviewFocused: boolean;
  onTogglePreviewFocus: () => void;
  path: string;
}
