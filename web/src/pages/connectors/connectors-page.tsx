import { ConnectorsDirectory } from "@/features/capability/connectors/connectors-directory";
import { WorkspacePageFrame } from "@/shared/ui/workspace-page-frame";

/** Connectors 页面 — 应用授权卡片网格 + 详情弹窗 */
export function ConnectorsPage() {
  return (
    <WorkspacePageFrame content_padding_class_name="p-0">
      <ConnectorsDirectory />
    </WorkspacePageFrame>
  );
}
