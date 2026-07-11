package channels

import (
	"context"
	"strings"
	"testing"
	"time"

	channeladapters "github.com/nexus-research-lab/nexus/internal/service/channels/adapters"
)

type fakePersonalWeixinLoginClient struct {
	status channeladapters.PersonalWeixinQRStatusResponse
}

func (c *fakePersonalWeixinLoginClient) StartQRCode(context.Context, []string) (channeladapters.PersonalWeixinQRCodeResponse, error) {
	return channeladapters.PersonalWeixinQRCodeResponse{
		QRCode:             "qr-token-1",
		QRCodeImageContent: "weixin://qr-login",
	}, nil
}

func (c *fakePersonalWeixinLoginClient) PollQRCodeStatus(context.Context, string, string) (channeladapters.PersonalWeixinQRStatusResponse, error) {
	if strings.TrimSpace(c.status.Status) != "" {
		return c.status, nil
	}
	return channeladapters.PersonalWeixinQRStatusResponse{
		Status:      "confirmed",
		BotToken:    "ilink-token-1",
		IlinkBotID:  "wx-account-1",
		IlinkUserID: "wx-user-1",
		BaseURL:     "https://ilink.test",
	}, nil
}

func waitChannelLoginStatus(
	t *testing.T,
	service *ControlService,
	ownerUserID string,
	channelType string,
	loginID string,
	status string,
) *ChannelLoginView {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		view, err := service.GetChannelLogin(context.Background(), ownerUserID, channelType, loginID)
		if err != nil {
			t.Fatalf("读取登录状态失败: %v", err)
		}
		if view.Status == status {
			return view
		}
		time.Sleep(10 * time.Millisecond)
	}
	view, err := service.GetChannelLogin(context.Background(), ownerUserID, channelType, loginID)
	if err != nil {
		t.Fatalf("读取最终登录状态失败: %v", err)
	}
	t.Fatalf("等待登录状态超时: got=%s want=%s view=%+v", view.Status, status, view)
	return nil
}

func testChannelCredentialKey() string {
	return "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
}
