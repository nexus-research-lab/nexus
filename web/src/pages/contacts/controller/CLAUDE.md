# Contacts 控制器

- 联系人集合只包含可管理的非主 Agent，编辑和删除命令不得再次判断另一套成员规则。
- 创建与更新复用 Agent Options 的字段投影和 mutation 参数。
- 编辑控制器直接生成 Agent Options Dialog 判别状态，不向页面暴露零散初值。
- 删除命令返回具体 Agent ID，是否离开当前路由由页面协调器决定。
