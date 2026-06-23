package logx

type field struct {
	key   string
	value string
}

type accessLog struct {
	method   string
	status   int
	duration string
	bytes    string
	path     string
}

type sdkDebugLog struct {
	summary    string
	sessionKey string
	agentID    string
	roundID    string
}
