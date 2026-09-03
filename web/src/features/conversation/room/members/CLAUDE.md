# Room 成员弹窗

## 职责边界

- `create-room-dialog.tsx` 只负责弹窗生命周期、区块组合和提交入口，消费已经完整化的具体参数。
- `room-member-manager-dialog.tsx` 统一桌面与手机入口的管理模式初始值、Agent 目录合并和提交关闭事务；各 Header/Surface 只维护与 `roomId` 绑定的打开状态。
- `create-room-dialog-model.ts` 负责可选参数默认化、弹窗重建身份和创建/管理标签投影，不持有 React 状态。
- 创建群聊入口可以使用 Lucide `MessageCirclePlus`，弹窗本身使用 plain Header，只让标题表达创建或管理模式。
- `use-create-room-form.ts` 独占表单状态、不变量归一化和提交模型构造；成员移除后群主失效、暂停草稿只保留已选成员等联动必须在这里完成。
- `room-settings-form.tsx`、`room-member-selector.tsx` 只负责各自视图和用户输入，不在渲染期修正状态；设置区以内容驱动的头像/名称与群主选项分组排布，不常驻渲染完整头像轨道；管理态成员行将增删与持久暂停/恢复呈现为两个独立动作。
- `room-avatar-picker.tsx` 只组合 Room 当前头像和共享锚定图标选择器，保持与 Agent 身份页一致的“明确入口后展开”交互。
- 创建与管理弹窗使用 plain Header、固定 plain Footer 和克制双栏，不用图标或副标题重复名称/成员要求；头像为 56px，桌面成员列表保持稳定视口并独立滚动，成员固定复用 `UiListRow density="dense"` 的单层浅底与 40px 行，暂停/恢复复用独立 `UiChoiceButton`，活动成员不绘制蓝色边框卡或圆形 plus。
- `skills/` 独占 Room 技能资源、选择状态和异步菜单，不将单一业务消费者伪装成共享控件。

## 约定

- 弹窗内容通过 React `key` 按初始值重建，禁止引入渲染期 `setState` 或逐字段重置。
- 可选 Props 只在纯模型解释一次，内容视图不得通过解构默认值重新形成分支协议。
- 创建与管理共用 `RoomDialogSubmission` 对象提交，禁止退回位置参数。
- 创建与管理标签通过静态 key 表投影，纯函数只接受该表覆盖范围内的窄翻译签名。
- 表单规则通过归一化数据结构表达，视图不得复制群主、成员和自动回复之间的约束。
- 管理模式只产生包含 `pausedAgentIds` 的完整提交对象；成员差异、参与状态差异和跨接口写入事务归页面命令层，弹窗不得直接调用 Room API。
- 成员暂停是 Room 级持久参与状态，不等同于停止当前输出；保存后由后端先收口该成员当前 slot，再闸住用户队列、Agent 唤醒、Goal continuation 与 WorkGraph dispatch，恢复时释放原样保留的工作。
- 管理弹窗在 `md` 以下使用内容驱动、带视口高度上限的单列布局，设置、成员和技能依次纵向排列；内容较少时不得强制撑满窗口，内容超高时只滚动 Body，也不得把桌面双栏压成窄条。
