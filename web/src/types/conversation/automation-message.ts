import type { Message } from "@/types/conversation/message/entity";

const AUTOMATION_TASK_MARKER_PATTERN = /(?:^|\n)\s*(?:-\s*)?\[(?:scheduled-task|cron):[^\]\r\n]+\]/;

/** 判断用户消息是否是自动化调度注入的内部触发。 */
export function isAutomationTriggerUserMessage(message: Message | undefined | null): boolean {
  if (!message || message.role !== "user" || typeof message.content !== "string") {
    return false;
  }
  return AUTOMATION_TASK_MARKER_PATTERN.test(message.content);
}
