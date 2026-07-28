package appregistration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestFeishuRegistrationUsesOfficialProtocol(t *testing.T) {
	pollCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil {
			t.Fatalf("解析飞书注册请求失败: %v", err)
		}
		switch request.Form.Get("action") {
		case "begin":
			if request.Form.Get("archetype") != "PersonalAgent" ||
				request.Form.Get("auth_method") != "client_secret" {
				t.Fatalf("飞书注册参数不正确: %v", request.Form)
			}
			_, _ = writer.Write([]byte(`{"device_code":"device-1","verification_uri":"https://scan.test/base","verification_uri_complete":"https://scan.test/complete","expires_in":600,"interval":1}`))
		case "poll":
			pollCount++
			if pollCount == 1 {
				writer.WriteHeader(http.StatusBadRequest)
				_, _ = writer.Write([]byte(`{"error":"authorization_pending"}`))
				return
			}
			_, _ = writer.Write([]byte(`{"client_id":"cli_auto","client_secret":"secret-auto","user_info":{"open_id":"ou_1","tenant_brand":"feishu"}}`))
		default:
			http.Error(writer, "unexpected action", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	client := NewFeishuClient(server.Client(), FeishuOptions{
		RegistrationURL: server.URL,
		Name:            "Nexus",
		TenantScopes:    []string{"im:message"},
		Events:          []string{"im.message.receive_v1"},
	})
	started, err := client.Start(context.Background())
	if err != nil {
		t.Fatalf("启动飞书应用注册失败: %v", err)
	}
	parsed, err := url.Parse(started.VerificationURIComplete)
	if err != nil {
		t.Fatalf("解析飞书二维码地址失败: %v", err)
	}
	if parsed.Query().Has("createOnly") || parsed.Query().Get("addons") == "" {
		t.Fatalf("飞书二维码应允许选择已有应用或创建新应用: %s", started.VerificationURIComplete)
	}
	pending, err := client.Poll(context.Background(), started.DeviceCode)
	if err != nil || pending.Status != StatusPending {
		t.Fatalf("飞书等待状态不正确: result=%+v err=%v", pending, err)
	}
	succeeded, err := client.Poll(context.Background(), started.DeviceCode)
	if err != nil || succeeded.Credentials["client_id"] != "cli_auto" {
		t.Fatalf("飞书注册凭据不正确: result=%+v err=%v", succeeded, err)
	}
}

func TestDingTalkRegistrationUsesOfficialProtocol(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("解析钉钉注册请求失败: %v", err)
		}
		switch request.URL.Path {
		case "/app/registration/init":
			if body["source"] != "DING_DWS_CLAW" {
				t.Fatalf("钉钉注册 source 不正确: %v", body)
			}
			_, _ = writer.Write([]byte(`{"errcode":0,"nonce":"nonce-1"}`))
		case "/app/registration/begin":
			_, _ = writer.Write([]byte(`{"errcode":0,"device_code":"device-1","verification_uri_complete":"https://ding.test/scan","expires_in":60,"interval":1}`))
		case "/app/registration/poll":
			_, _ = writer.Write([]byte(`{"errcode":0,"status":"SUCCESS","client_id":"ding-client","client_secret":"ding-secret"}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client := NewDingTalkClient(server.Client(), server.URL)
	started, err := client.Start(context.Background())
	if err != nil || started.VerificationURIComplete != "https://ding.test/scan" {
		t.Fatalf("启动钉钉注册失败: result=%+v err=%v", started, err)
	}
	result, err := client.Poll(context.Background(), started.DeviceCode)
	if err != nil || result.Credentials["client_secret"] != "ding-secret" {
		t.Fatalf("钉钉注册凭据不正确: result=%+v err=%v", result, err)
	}
}

func TestWeComRegistrationUsesOfficialProtocol(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/ai/qc/generate":
			if request.URL.Query().Get("source") != "wecom-cli" {
				t.Fatalf("企业微信注册 source 不正确: %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"data":{"scode":"scode-1","auth_url":"https://wecom.test/scan"}}`))
		case "/ai/qc/query_result":
			if !strings.EqualFold(request.URL.Query().Get("scode"), "scode-1") {
				t.Fatalf("企业微信轮询 scode 不正确")
			}
			_, _ = writer.Write([]byte(`{"data":{"status":"success","bot_info":{"botid":"bot-1","secret":"secret-1"}}}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client := NewWeComClient(server.Client(), server.URL)
	started, err := client.Start(context.Background())
	if err != nil || started.VerificationURIComplete != "https://wecom.test/scan" {
		t.Fatalf("启动企业微信扫码绑定失败: result=%+v err=%v", started, err)
	}
	result, err := client.Poll(context.Background(), started.DeviceCode)
	if err != nil || result.Credentials["bot_id"] != "bot-1" {
		t.Fatalf("企业微信扫码凭据不正确: result=%+v err=%v", result, err)
	}
}
