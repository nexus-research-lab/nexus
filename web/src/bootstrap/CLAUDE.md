# 应用启动

- `root-bootstrap.tsx` 只编排文档标记、主题、应用级运行时配置命令、首次渲染和 watchdog 启动。
- `root-renderer.tsx` 拥有 React Root 和渲染命令，`root-failure-view.tsx` 只导出错误边界与失败视图。
- `recovery/` 负责全局错误、chunk/auth 恢复、重载哨兵和空白渲染健康检查。
- 各构建入口只调用 `bootstrapReactApp`，不得复制启动阶段或绕过根错误边界。
- 运行时配置的请求与快照提交归 `app/runtime-options-resource.ts`，Bootstrap 不直接解释响应字段。
