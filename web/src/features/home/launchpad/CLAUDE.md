# Home Launchpad

## 职责边界

- `home-launch-model.ts`：纯投影。时段划分、进行中会话（只取等你回复与执行中的，排除主智能体与空闲会话，最多三条）、接下来的定时任务、新手示例与示例显隐阈值。
- `use-home-launch-controller.ts`：读取 Home 共享目录、Room 活动快照和定时任务目录；提交时通过 Direct Room 的 `initial` 草稿协议进入主智能体 DM，由该 Composer 投递到 inbox，首页不自行发送消息。
- `home-launchpad.tsx`：直接复用共享 `ComposerPanel`（草稿作用域 `home:main-agent`），以及与字标同一居中轴的窄列：问候、示例与「正在进行」列表。除输入框外不画边框，列表行只在悬停时给底色，且不重复侧栏已有的会话列表与导航入口。Footer 的权限、模型与连接器绑定主智能体已有 Session；附件上传到主智能体工作区后经 `composer-initial-attachments.ts` 随 initial 草稿交接；Slash、队列与 Goal 依赖会话实时通道，首页不提供。

## 约定

- 示例卡通过 Composer 草稿 Store 写入首页作用域，只填入不发送。
- 定时任务读取失败只隐藏其中的任务行，不影响输入与进行中的会话。
