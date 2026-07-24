// Package projectpermission 提供共享项目 ACL 控制面 HTTP handlers。
//
// 所有路径和成员变更最终交给宿主 launcher；请求层不直接触碰 OS 组或 ACL。
package projectpermission
