// INPUT: Channel 保存、扫码、删除与等待状态及对应动作。
// OUTPUT: plain 弹窗底部的断开、取消和单一主动作。
// POS: Channel 连接弹窗的动作投影，不解释状态机或重复表单内容。
import { Loader2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiDialogFooter } from "@/shared/ui/dialog/dialog";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";

import {
  getChannelSubmitLabel,
  type ChannelSubmitState,
} from "./channel-connect-dialog-model";

interface ChannelConnectDialogFooterProps extends ChannelSubmitState {
  agentId: string;
  busy: boolean;
  closeBlocked: boolean;
  configured: boolean;
  deleting: boolean;
  onCancel: () => void;
  onRequestDelete: () => void;
}

export function ChannelConnectDialogFooter({
  agentId,
  busy,
  closeBlocked,
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
  const { t } = useI18n();
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
              ? <Loader2 className={getUiSpinnerClassName({ size: "sm" })} />
              : null}
            {t(deleting ? "capability.channel_disconnecting" : "capability.channel_disconnect")}
          </UiButton>
        ) : null}
      </div>
      <div className="flex justify-end gap-2">
        <UiButton
          disabled={closeBlocked || deleting}
          onClick={onCancel}
          size="sm"
          type="button"
        >
          {t("common.cancel")}
        </UiButton>
        <UiButton
          disabled={submitDisabled}
          size="sm"
          tone="primary"
          type="submit"
          variant="solid"
        >
          {getChannelSubmitLabel(submitState, t)}
        </UiButton>
      </div>
    </UiDialogFooter>
  );
}
