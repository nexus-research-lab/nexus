// INPUT: Channel 保存、扫码、删除与等待状态及对应动作。
// OUTPUT: plain 弹窗底部的断开、取消和单一主动作。
// POS: Channel 连接弹窗的动作投影，不解释状态机或重复表单内容。
import { Loader2 } from "lucide-react";

import { UiButton } from "@/shared/ui/button/button";
import { UiDialogFooter } from "@/shared/ui/dialog/dialog";

import {
  getChannelSubmitLabel,
  type ChannelSubmitState,
} from "./channel-connect-dialog-model";

interface ChannelConnectDialogFooterProps extends ChannelSubmitState {
  agentId: string;
  busy: boolean;
  configured: boolean;
  deleting: boolean;
  onCancel: () => void;
  onRequestDelete: () => void;
}

export function ChannelConnectDialogFooter({
  agentId,
  busy,
  configured,
  deleting,
  loginLoading,
  loginRunning,
  onCancel,
  onRequestDelete,
  planned,
  saving,
  supportsQRCode,
}: ChannelConnectDialogFooterProps) {
  const submitState: ChannelSubmitState = {
    loginLoading,
    loginRunning,
    planned,
    saving,
    supportsQRCode,
  };
  const submitDisabled = busy || loginRunning || !agentId || planned;

  return (
    <UiDialogFooter appearance="plain" className="justify-between">
      <div>
        {configured && !planned ? (
          <UiButton
            disabled={busy}
            onClick={onRequestDelete}
            size="sm"
            tone="danger"
            type="button"
            variant="text"
          >
            {deleting
              ? <Loader2 className="h-3.5 w-3.5 animate-spin" />
              : null}
            {deleting ? "断开中..." : "断开频道"}
          </UiButton>
        ) : null}
      </div>
      <div className="flex justify-end gap-2">
        <UiButton
          disabled={deleting}
          onClick={onCancel}
          size="sm"
          type="button"
        >
          取消
        </UiButton>
        <UiButton
          disabled={submitDisabled}
          size="sm"
          tone="primary"
          type="submit"
          variant="solid"
        >
          {getChannelSubmitLabel(submitState)}
        </UiButton>
      </div>
    </UiDialogFooter>
  );
}
