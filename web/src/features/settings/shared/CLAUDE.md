# shared/ - 设置与运营共享 UI

- `settings-panel-ui.tsx` 只保存设置型表面的布局常量和通用分段控件。
- General、Operations 等消费者可依赖这里；共享层不得反向依赖具体设置子域。
