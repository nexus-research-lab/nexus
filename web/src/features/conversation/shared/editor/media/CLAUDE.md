# 原生媒体与 HTML 预览

- `media-file-preview.tsx` 的 PDF/Image 入口保持路由类型职责，内部共用原生内容、标题栏和加载组合；图片的内在比例/留白仍是专用展示，失败面可在有限高度滚动。二进制占位继续按当前语言/宿主说明已有文件操作。
- `use-native-media-preview.ts` 拥有 owner 代次、Agent/path 与每次显式重新加载的事件作用域；切换后返回同一文件也不能复用旧回调。Chrome 文案或专注状态变化保留原生元素；文件/账号变化或重新加载才生成新元素。`settled` 只表示收到 load 事件，不是成功读取文件的业务证据。
- 图片使用原生 error 事实提供重试。PDF iframe 不依赖 error 事件，不推断内容成败；工具栏始终提供公共按钮的显式重新加载，load 只清除等待。事件行为依据：[MDN iframe](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/iframe#error_and_load_event_behavior)。两个沙箱和原生 URL 的现有权限保持。
- `html-file-preview.tsx` 保留 opaque-origin iframe 与内存 storage shim；单个 effect 拥有流式提交的定时器、更新和卸载清理，最终内容即时提交。Head 未完成且尚无已提交文档时，使用共享源码度量和具名可聚焦滚动区域；已有文档不会被半截 Head 替换。
- `media-file-preview.test.tsx` 验证实例/事件作用域、图片重试和 PDF 重新加载；`html-file-preview.test.tsx` 验证 source/iframe 切换、shim 插入顺序、沙箱属性、节流/收尾/清理。测试不执行 frame 脚本、加载原生 PDF 或替代宿主视觉验收。
