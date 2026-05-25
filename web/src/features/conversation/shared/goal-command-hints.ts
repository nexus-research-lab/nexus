import type { ComposerCommandHintItem } from "./composer-command-hint-model";

export const GOAL_COMMAND_HINT_ITEMS: ComposerCommandHintItem[] = [
  {
    command: "/goal <objective>",
    detail: "创建或替换当前 Goal",
    insert_text: "/goal ",
  },
  {
    command: "/goal",
    detail: "查看当前 Goal",
  },
  {
    command: "/goal pause",
    detail: "暂停当前 Goal",
  },
  {
    command: "/goal resume",
    detail: "继续当前 Goal",
  },
  {
    command: "/goal clear",
    detail: "清除当前 Goal",
  },
];
