export interface WorkspaceFilePreviewProps {
  agentId: string;
  embedded?: boolean;
  fileName: string;
  isPreviewFocused?: boolean;
  onResizeStart: () => void;
  onTogglePreviewFocus?: () => void;
  path: string;
}
