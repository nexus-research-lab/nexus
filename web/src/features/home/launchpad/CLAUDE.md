# Home Launchpad

## 职责边界

- `home-launch-model.ts`：纯投影。时段划分、继续区排序（等你回复 > 进行中 > 最近活动，排除主智能体）、接下来的定时任务、新手示例与示例显隐阈值。
- `use-home-launch-controller.ts`：读取 Home 共享目录、Room 活动快照和定时任务目录；提交时通过 Direct Room 的 `initial` 草稿协议进入主智能体 DM，由该 Composer 投递到 inbox，首页不自行发送消息。
- `home-launchpad.tsx`：输入框、快捷入口、继续区、接下来与示例卡视图。

## 约定

- 示例卡只填入草稿，不直接发送。
- 定时任务读取失败只隐藏「接下来」，不影响输入与继续区。
