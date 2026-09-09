# Workspace File Tree

供 Workspace 目录面复用的文件树，当前由 Room 文件浏览器消费。

## 职责

- `workspace-file-tree-model.ts`：文件层级、Material 文件图标映射和单行选中/展开展示的纯投影。
- `workspace-file-tree.tsx`：公共入口，构建树、稳定动作与按路径保存的展开偏好；顶层默认打开，父级收起不清除子级选择，同目录快照刷新保留选择，已经消失的目录记录清除。只渲染可见子树，避免保留隐藏 DOM。
- `workspace-file-tree-row.tsx`：受控递归行、指示器、子树与行内动作视图；不在易被卸载的子行保存展开真相。目录按嵌套 list/disclosure 与原生 Tab、Enter/Space 导航，不声明缺少对应键盘状态机的 tree widget。

## 边界

- 文件树只消费 `WorkspaceFileEntry` 和调用方动作，不读取 Room 状态或调用 API。
- 文件名与扩展名规则使用数据表维护，视图只消费已解析的 SVG 资源，不增加类型分支。
- 递归层只传一个稳定动作对象，避免每层扩散同组回调。

- 文件/目录主入口是该 Tree owner 的透明原生 disclosure button，整行独占 hover/selected 背景，展开不等于选中；重命名/删除收进一个 UiListActionButton 触发的 UiActionMenu；文件管理命令只接收原 entry。次动作按完整路径具名，在行 hover/focus 或无 hover 设备可见，选中文件的动作常显；公共 owner 拥有按钮色彩、尺寸、禁用、焦点与唯一 Tooltip。
- 行名使用 supporting/regular，目录或选中不加粗；文件宽度服从父容器，长名称省略并在主按钮保留完整路径 title。层级缩进维持 8px + 每层 12px，但最多占行宽 35%，为名称和行次动作留下空间。
- 无列表标记时仍显式保留 list 语义，根具名，嵌套列表关联父目录入口；目录暴露展开状态和存在时的 controls，当前文件暴露 aria-current，装饰图标不参与命名。
- 两个去掉标记的 `ul` 按 [WebKit 的列表启发式说明](https://bugs.webkit.org/show_bug.cgi?id=170179#c1) 显式设置 `role="list"`，仅在对应行说明 `no-redundant-roles` 例外，不放宽全局检查；此实现依据不等同于已完成宿主验收。
