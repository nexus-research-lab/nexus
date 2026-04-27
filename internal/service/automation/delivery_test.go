package automation

import "testing"

func TestFilterHeartbeatResponseSuppressesAckOnlyReply(t *testing.T) {
	result := filterHeartbeatResponse("HEARTBEAT_OK", 300)
	if result.ShouldDeliver {
		t.Fatalf("纯 HEARTBEAT_OK 不应外发: %+v", result)
	}
	if result.Text != "" {
		t.Fatalf("纯 HEARTBEAT_OK 应清空文本，实际 %q", result.Text)
	}
}

func TestFilterHeartbeatResponseKeepsLongAlertText(t *testing.T) {
	result := filterHeartbeatResponse("HEARTBEAT_OK\nalert: disk space is low", 8)
	if !result.ShouldDeliver {
		t.Fatalf("超出阈值的告警文本应外发: %+v", result)
	}
	if result.Text != "alert: disk space is low" {
		t.Fatalf("告警文本提取错误: %q", result.Text)
	}
}

func TestFilterHeartbeatResponseRespectsAckThreshold(t *testing.T) {
	shortResult := filterHeartbeatResponse("HEARTBEAT_OK\nwarn", 4)
	longResult := filterHeartbeatResponse("HEARTBEAT_OK\nwarn", 3)
	if shortResult.ShouldDeliver || shortResult.Text != "" {
		t.Fatalf("等于阈值时不应外发: %+v", shortResult)
	}
	if !longResult.ShouldDeliver || longResult.Text != "warn" {
		t.Fatalf("超过阈值时应外发剩余文本: %+v", longResult)
	}
}
