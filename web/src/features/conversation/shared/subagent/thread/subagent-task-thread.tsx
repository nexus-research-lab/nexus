"use client";

import { useEffect, useState } from "react";

import { ConfirmDialog, PromptDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { SubagentTask, SubagentTaskSource } from "@/types/conversation/subagent-task";

import {
	canSendSubagentTaskMessage,
	isSubagentTaskActive,
	subagentTaskSourceKey,
} from "../subagent-task-model";
import { SubagentTaskThreadView } from "./subagent-task-thread-view";
import { useSubagentTaskThread } from "./use-subagent-task-thread";

interface SubagentTaskDialog {
	kind: "send" | "stop";
	scopeKey: string;
}

interface SubagentTaskThreadProps {
  layout?: "desktop" | "mobile";
  onBack: () => void;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  source: SubagentTaskSource;
  task: SubagentTask;
}

export function SubagentTaskThread({
  layout = "desktop",
  onBack,
  onOpenWorkspaceFile,
  source,
  task,
}: SubagentTaskThreadProps) {
  const thread = useSubagentTaskThread({ source, task });
	const { t } = useI18n();
	const scopeKey = `${subagentTaskSourceKey(source)}:${task.task_id}`;
	const [dialog, setDialog] = useState<SubagentTaskDialog | null>(null);
	const closeDialog = () => setDialog(null);
	const dialogMatchesScope = dialog?.scopeKey === scopeKey;
	useEffect(() => {
		setDialog(null);
	}, [scopeKey]);

  return (
		<>
			<SubagentTaskThreadView
				layout={layout}
				model={{
					...thread,
					onRetry: () => void thread.refresh(),
					onSendRequest: () => setDialog({ kind: "send", scopeKey }),
					onStopRequest: () => setDialog({ kind: "stop", scopeKey }),
				}}
				onBack={onBack}
				onOpenWorkspaceFile={onOpenWorkspaceFile}
			/>
			<ConfirmDialog
				cancelText={t("common.cancel")}
				confirmText={t("subagents.stop_confirm")}
				isOpen={dialog?.kind === "stop"
					&& dialogMatchesScope
					&& isSubagentTaskActive(thread.task)
					&& thread.task.capabilities.stop}
				message={t("subagents.stop_description")}
				onCancel={closeDialog}
				onConfirm={() => {
					if (!dialogMatchesScope) {
						closeDialog();
						return;
					}
					closeDialog();
					void thread.actions.stop();
				}}
				subtitle={t("subagents.stop_subtitle")}
				title={t("subagents.stop_title")}
				variant="danger"
			/>
			<PromptDialog
				cancelText={t("common.cancel")}
				confirmText={t("subagents.message_send")}
				isOpen={dialog?.kind === "send"
					&& dialogMatchesScope
					&& canSendSubagentTaskMessage(thread.task)}
				message={t("subagents.message_description")}
				multiline
				onCancel={closeDialog}
				onConfirm={(message) => {
					if (!dialogMatchesScope || !message.trim()) {
						if (!dialogMatchesScope) {
							closeDialog();
						}
						return;
					}
					closeDialog();
					void thread.actions.send(message);
				}}
				placeholder={t("subagents.message_placeholder")}
				rows={5}
				shortcutHint={t("subagents.message_shortcut_hint")}
				title={t("subagents.message_title")}
			/>
		</>
  );
}
