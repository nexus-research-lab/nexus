import type { Message } from "@/types/conversation/message";

const AUTOMATION_CRON_MARKER_PATTERN = /(?:^|\n)\s*(?:-\s*)?\[cron:[^\]\r\n]+\]/;

/** 判断用户消息是否是自动化调度注入的内部触发。 */
export function isAutomationTriggerUserMessage(message: Message | undefined | null): boolean {
  if (!message || message.role !== "user" || typeof message.content !== "string") {
    return false;
  }
  return AUTOMATION_CRON_MARKER_PATTERN.test(message.content);
}
