# WorkGraph picker

- `workgraph-distillation-picker-dialog.tsx` 持有开放作用域的 owner 目录读取、搜索、选择与只读预览；每次打开从当前目录重新建立选择，读取不依赖来源 Session，不保留未使用的 Session 参数。
- 搜索初始焦点交给公共 Dialog；列表使用共享 SelectMenuOptionRow，选中项是唯一 Tab 入口，方向键与 Home/End 同时移动焦点和预览，浏览不写命令。明确点击使用才把原始 `/<slash_name> ` 交给当前 Composer 并关闭。
- UiPanel 拥有目录/预览外边界，公共 Typography 拥有标题、命令及说明字号；桌面双栏、窄屏纵向排列，完整结构由 NamedWorkGraphSketch 展示。
- 已有快照的普通刷新失败保留目录，访问失效隐藏选项，筛选空集和首次空目录保留各自原有恢复路径。关闭开放作用域后忽略迟到读取；不创建或保存工作图、不启动 Execution。
