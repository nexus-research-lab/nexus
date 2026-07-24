package projectpermission

// Project 描述一个由 launcher 管理的共享项目。
type Project struct {
	ProjectID  string            `json:"project_id"`
	GroupName  string            `json:"group_name"`
	GID        int               `json:"gid"`
	Root       string            `json:"root"`
	Members    map[string]string `json:"members"`
	Generation uint64            `json:"generation"`
}

// EnsureResult 表示项目 ensure 是否首次创建。
type EnsureResult struct {
	Project Project `json:"project"`
	Created bool    `json:"created"`
}

// GrantResult 表示成员 ACL 是否发生了实际变化。
type GrantResult struct {
	Changed bool `json:"changed"`
}
