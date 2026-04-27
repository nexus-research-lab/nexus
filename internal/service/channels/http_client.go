package channels

import (
	"net/http"
	"time"
)

var defaultChannelHTTPClient = &http.Client{Timeout: 45 * time.Second}
