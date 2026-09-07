# attachments/

L5 | 父级: web/src/features/conversation/shared/composer

## 职责

- `composer-attachments.ts`: 以有序规则表统一附件分类、文件选择过滤、批量校验和上传投影
- `composer-local-attachment-model.ts`: 管理剪贴板动作、本地标识、批次投影和待发送附件模型
- `use-composer-attachments.ts`: 按动作表执行粘贴策略，消费草稿胶囊中的受控附件，并管理错误翻译和发送前准备生命周期
- `composer-local-attachments.tsx`: 展示可点击图片缩略图，以及共享 RemovableChip 承载的文本/普通文件胶囊；移除名称包含文件名，原生 title 只显示文件名，不保留私有中文类型表
- `composer-attachment-preview-dialog.tsx`: 图片与文本共用一个 Dialog/Header，由公共 Header 自动生成并关联实例标题；正文按附件 ID 隔离，仅显示文件名，保留图片灯箱和最多 512 KiB 的只读文本预览
- `use-composer-local-file-url.ts`: 独占图片缩略图和灯箱使用的 Object URL 创建与释放；File 替换时同步隐藏旧 URL，再由 effect 分配新资源

附件批次必须先完整校验，再产生上传副作用，避免留下半批资源。
Agent Workspace 与 Room Conversation 只提供上传目标和作用域字段，不复制分类规则或上传循环。
图片与文本预览属于瞬时 UI；切换 Session 时同步关闭，附件移除后清空预览选择，恢复同一草稿也不自动重开。草稿附件本身仍按 Room/DM 作用域恢复。预览按钮和移除按钮必须保持独立，不能由冒泡导致误删除或误打开。
图片和文本预览统一使用 `UiButton`；普通文件/文本的移除、字号和胶囊几何由 `UiRemovableChip size="xs"` 拥有。图片角上的 `2xs` 移除层只定义定位，复用共享的圆形、danger 与 surface 状态。48px 图片框属于内容缩略图几何，允许保留覆盖动作，不复制普通胶囊或按钮 DOM。
附件 Chip 与输入壳的圆角由共享 recipe 唯一拥有；Composer 样式只保留自身壳层标记和布局，不重复覆盖共享值。共置测试覆盖独立命令、Session 预览重置与 Object URL 释放，浏览器夹具验证真实文件解码、预览/移除和键盘焦点。
附件文件名是预览唯一可见标题；不得再重复“图片预览”“文本预览”等可由内容直接判断的类型说明。
图片失败随 File 重置；文本读取使用 512 KiB slice 并取消过期提交。加载/失败共用一处 ResourceState，短面板由独立滚动面承载恢复动作；只读文本复用公共源码排版与预览视口，以文件名命名且可 Tab 聚焦。当前离线测试只提供 File 读入和原生图片事件夹具，不替代暂停中的真实解码/宿主验收。
协议拒绝结果使用结构化错误码，用户文案由 Composer 的 i18n 消费层统一生成。
剪贴板只投影为原生粘贴、追加文件、追加长文本或拒绝 Goal 附件四种动作，Hook 不维护条件链。
