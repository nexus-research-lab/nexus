import {
  Loader2,
  Power,
  QrCode,
  Trash2,
} from "lucide-react";

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
  supportsPersonalWeixinLogin,
}: ChannelConnectDialogFooterProps) {
  const submitState: ChannelSubmitState = {
    loginLoading,
    loginRunning,
    planned,
    saving,
    supportsPersonalWeixinLogin,
  };
  const submitDisabled = busy || loginRunning || !agentId || planned;

  return (
    <UiDialogFooter>
      <div className="flex w-full flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-h-10">
          {configured && !planned ? (
            <UiButton
              className="min-w-[118px]"
              disabled={busy}
              onClick={onRequestDelete}
              size="lg"
              tone="danger"
              type="button"
            >
              {deleting
                ? <Loader2 className="h-5 w-5 animate-spin" />
                : <Trash2 className="h-5 w-5" />}
              {deleting ? "断开中..." : "断开频道"}
            </UiButton>
          ) : null}
        </div>
        <div className="flex justify-end gap-3">
          <UiButton
            className="min-w-[104px]"
            disabled={deleting}
            onClick={onCancel}
            size="lg"
            type="button"
          >
            取消
          </UiButton>
          <UiButton
            className="min-w-[124px]"
            disabled={submitDisabled}
            size="lg"
            tone="primary"
            type="submit"
            variant="solid"
          >
            {supportsPersonalWeixinLogin
              ? <QrCode className="h-5 w-5" />
              : <Power className="h-5 w-5" />}
            {getChannelSubmitLabel(submitState)}
          </UiButton>
        </div>
      </div>
    </UiDialogFooter>
  );
}
