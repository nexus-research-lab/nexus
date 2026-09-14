# Connector 页面入口

- 目录页只委托 ConnectorsDirectory，不复制目录状态或详情。
- OAuth 回调仅显示受控状态，不回显 code、state、Provider 错误正文或凭据；完成连接只消费一次。
- 状态保存 capability 目录翻译键，切换语言重新投影文案而不重新提交 OAuth；事件仍携带受控翻译文案。
- 标题、正文和关闭提示复用公共 Typography；回调 transport 与 Desktop 返回链路保持独立。
