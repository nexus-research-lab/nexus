// INPUT: Channel 配置快照、Agent 列表与保存/删除动作。
// OUTPUT: plain 标题、可折叠接入说明和配置字段组成的连接弹窗。
// POS: Channel 配置主弹窗，只编排表单与确认，不展开业务请求实现。
"use client";

import type { FormEvent } from "react";

import type { ChannelConfigView } from "@/lib/api/capability/channel-api";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import type { Agent } from "@/types/agent/agent";

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
              appearance="plain"
              onClose={onClose}
              title={`连接 ${controller.currentItem.title}`}
              titleId="channel-connect-dialog-title"
            />

            <UiDialogBody className="space-y-5 px-5" scrollable>
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
              supportsQRCode={controller.offersQRCode}
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
