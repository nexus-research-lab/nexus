"use client";

import type { FormEvent } from "react";

import type { ChannelConfigView } from "@/lib/api/capability/channel-api";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import type { Agent } from "@/types/agent/agent";

import { ChannelIcon } from "../channel-icon";
import { useChannelConnectionController } from "./use-channel-connection-controller";
import { ChannelConnectDialogFooter } from "./view/channel-connect-dialog-footer";
import { getChannelDeleteDialogCopy } from "./view/channel-connect-dialog-model";
import { ChannelConnectionFields } from "./view/channel-connection-fields";

interface ChannelConnectDialogProps {
  agents: Agent[];
  item: ChannelConfigView;
  onClose: () => void;
  onDeleted: (item: ChannelConfigView) => Promise<void> | void;
  onError: (message: string) => void;
  onSaved: (item: ChannelConfigView, announce?: boolean) => void;
}

export function ChannelConnectDialog({
  agents,
  item,
  onClose,
  onDeleted,
  onError,
  onSaved,
}: ChannelConnectDialogProps) {
  const controller = useChannelConnectionController({
    agents,
    item,
    onClose,
    onDeleted,
    onError,
    onSaved,
  });
  const deleteCopy = getChannelDeleteDialogCopy(
    controller.pendingDelete,
    controller.currentItem,
  );

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault();
    void controller.saveChannel();
  };

  return (
    <>
      <UiDialogPortal>
        <UiDialogBackdrop
          className="z-[9999]"
          labelledBy="channel-connect-dialog-title"
          onClose={onClose}
        >
          <UiDialogFormShell
            autoComplete="off"
            className="max-h-[86vh]"
            onSubmit={handleSubmit}
            size="lg"
          >
            <UiDialogHeader
              icon={<ChannelIcon type={controller.currentItem.channel_type} size="dialog" />}
              iconClassName="h-[52px] w-[52px] overflow-visible border-0 bg-transparent p-0 shadow-none"
              onClose={onClose}
              title={`连接 ${controller.currentItem.title}`}
              titleId="channel-connect-dialog-title"
            />

            <UiDialogBody className="space-y-5" scrollable>
              {controller.planned ? (
                <UiStateBlock
                  description="频道接入将在后续版本补充，当前版本暂不支持配置机器人或配对。"
                  size="sm"
                  title="该频道未上线"
                  variant="inset"
                />
              ) : (
                <ChannelConnectionFields
                  agents={agents}
                  controller={controller}
                />
              )}
            </UiDialogBody>

            <ChannelConnectDialogFooter
              agentId={controller.draft.agentId}
              busy={controller.busy}
              configured={controller.currentItem.configured}
              deleting={controller.deleting}
              loginLoading={controller.loginLoading}
              loginRunning={controller.loginRunning}
              onCancel={onClose}
              onRequestDelete={controller.requestDeleteChannel}
              planned={controller.planned}
              saving={controller.saving}
              supportsPersonalWeixinLogin={controller.supportsPersonalWeixinLogin}
            />
          </UiDialogFormShell>
        </UiDialogBackdrop>
      </UiDialogPortal>
      <ConfirmDialog
        confirmText={deleteCopy.confirmText}
        isOpen={controller.pendingDelete !== null}
        message={deleteCopy.message}
        onCancel={() => controller.setPendingDelete(null)}
        onConfirm={controller.confirmDelete}
        title={deleteCopy.title}
        variant="danger"
      />
    </>
  );
}
