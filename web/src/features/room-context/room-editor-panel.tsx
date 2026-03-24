import { ComponentProps } from "react";

import { WorkspaceEditorPane } from "@/components/workspace/workspace-editor-pane";

export type RoomEditorPanelProps = ComponentProps<typeof WorkspaceEditorPane>;

export function RoomEditorPanel(props: RoomEditorPanelProps) {
  return <WorkspaceEditorPane {...props} />;
}
